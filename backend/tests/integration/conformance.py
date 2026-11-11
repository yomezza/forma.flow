"""Exercise every OpenAPI operation against a real HTTP server and PostgreSQL.
Dependencies are developer-only: tests/requirements.txt.
"""
import http.cookiejar
import json
import os
from pathlib import Path
import re
import urllib.error
import urllib.request
import uuid
import yaml
from jsonschema import Draft202012Validator, FormatChecker

spec = yaml.safe_load((Path(__file__).parents[2] / "api/openapi.yaml").read_text())
base = os.environ["CONFORMANCE_URL"]
origin = "http://forma.test"
covered = set()
operations = []
for path, methods in spec["paths"].items():
    for method, operation in methods.items():
        pattern = "^" + re.sub(r"\{[^}]+\}", "[^/]+", path) + "$"
        operations.append((method.upper(), path, re.compile(pattern), operation))

def client():
    return {"http": urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())), "csrf": ""}

owner = client()
specialist = client()
public = client()

def check(schema, value):
    root = {"components": spec["components"], **schema}
    Draft202012Validator(root, format_checker=FormatChecker()).validate(value)

def call(method, path, data=None, user=owner, expected=None, headers=None):
    candidates = [row for row in operations if row[0] == method and row[2].match(path.split("?", 1)[0])]
    assert candidates, (method, path)
    _, template, _, operation = max(candidates, key=lambda row: len(re.sub(r"\{[^}]+\}", "", row[1])))
    if data is not None:
        check(operation["requestBody"]["content"]["application/json"]["schema"], data)
    h = {"Origin": origin, "Content-Type": "application/json"}
    if user["csrf"]:
        h["X-CSRF-Token"] = user["csrf"]
    h.update(headers or {})
    req = urllib.request.Request(base + path, data=None if data is None else json.dumps(data).encode(), headers=h, method=method)
    try:
        response = user["http"].open(req)
    except urllib.error.HTTPError as e:
        response = e
    code = response.code
    raw = response.read()
    result = json.loads(raw) if raw else None
    if expected is not None:
        assert code == expected, (operation["operationId"], code, result)
    else:
        assert 200 <= code < 300, (operation["operationId"], code, result)
    assert str(code) in operation["responses"], (operation["operationId"], "undocumented HTTP", code)
    schema = operation["responses"][str(code)].get("content", {}).get("application/json", {}).get("schema")
    if schema:
        check(schema, result)
    if code < 300:
        covered.add(operation["operationId"])
    if result and "csrfToken" in result:
        user["csrf"] = result["csrfToken"]
    assert response.headers.get("Cache-Control") == "no-store"
    assert response.headers.get("X-Request-ID")
    return result

def clock(value):
    req = urllib.request.Request(os.environ["CLOCK_CONTROL_URL"], data=json.dumps({"now": value}).encode(), method="POST")
    urllib.request.urlopen(req).close()

def mode_revision():
    return call("GET", "/api/v1/admin/schedule/mode")["revision"]

call("POST", "/api/v1/auth/sessions", {"email": "owner@forma.test", "password": "test-owner-password-long"})
call("GET", "/api/v1/auth/sessions/current")
call("GET", "/api/v1/admin/clients", user=public, expected=401)
call("GET", "/api/v1/services/missing", user=public, expected=404)
call("POST", "/api/v1/auth/sessions", {"email":"owner@forma.test","password":"test-owner-password-long"}, user=public, expected=403, headers={"Origin":"https://untrusted.invalid"})
call("GET", "/api/v1/services", user=public)
call("GET", "/api/v1/services/A", user=public)
settings = call("GET", "/api/v1/site/settings", user=public)
content = call("GET", "/api/v1/site/content", user=public)
call("GET", "/api/v1/admin/settings")
call("PATCH", "/api/v1/admin/settings", settings)
call("PATCH", "/api/v1/admin/site/content", content)
call("GET", "/api/v1/admin/services")
c = call("POST", "/api/v1/admin/services", {"id": "C", "name": "Massage C", "description": "Test", "durationMinutes": 30, "priceAmount": 1000, "currency": "RUB", "sortOrder": 3, "isActive": True, "visualSymbol": "logo-01"})
call("PATCH", "/api/v1/admin/services/C", {"name": "Updated C"})
call("DELETE", "/api/v1/admin/services/C")
call("GET", "/api/v1/availability/dates?from=2027-01-02&to=2027-01-03&serviceId=A", user=public)
call("GET", "/api/v1/availability/slots?date=2027-01-02&serviceId=A", user=public)
b = call("POST", "/api/v1/booking-requests", {"serviceId": "A", "date": "2027-01-02", "time": "10:00", "fullName": "Иван Петров", "phone": "+79991234567", "consentAccepted": True, "consentVersion": "v1"}, public, headers={"Idempotency-Key": str(uuid.uuid4())})
bpath = "/api/v1/admin/bookings/" + b["id"]
call("GET", "/api/v1/admin/bookings")
call("GET", "/api/v1/admin/dashboard")
call("GET", "/api/v1/admin/attention")
c = call("POST", "/api/v1/admin/clients", {"fullName": "Иван Петров", "phone": "+79991234567"})
cp = "/api/v1/admin/clients/" + c["id"]
call("PATCH", cp, {"fullName": "Иван Иванович Петров"})
call("GET", cp)
call("GET", "/api/v1/admin/clients")
call("GET", "/api/v1/admin/clients/by-phone?phone=%2B79991234567")
call("PUT", bpath + "/client", {"clientId": c["id"]})
call("GET", cp + "/bookings")
call("POST", bpath + "/confirm")
s = call("POST", "/api/v1/admin/specialists", {"fullName": "Тестовый Специалист", "serviceIds": ["A", "B"], "accountId": None})
sp = "/api/v1/admin/specialists/" + s["id"]
call("GET", "/api/v1/admin/specialists")
call("GET", sp)
call("PATCH", sp, {"fullName": "Опытный Специалист"})
schedule = call("GET", sp + "/schedule")
schedule["week"] = {str(i): [{"start": "09:00", "end": "19:00"}] for i in range(7)}
call("PUT", sp + "/schedule", schedule)
invitation = call("POST", "/api/v1/admin/staff-invitations", {"email": "conformance@forma.test", "roles": ["admin", "specialist"], "specialistId": s["id"]})
call("POST", "/api/v1/auth/sessions", {"email": "conformance@forma.test", "password": invitation["temporaryPassword"]}, specialist)
call("POST", "/api/v1/auth/password/change", {"currentPassword": invitation["temporaryPassword"], "newPassword": "new-conformance-password-long"}, specialist)
candidates = call("GET", bpath + "/assignment/candidates")
assignment = {"specialistId": s["id"], "expectedCapacityRevision": candidates["capacityRevision"]}
call("POST", bpath + "/assignment/preview", assignment)
call("PUT", bpath + "/assignment", assignment)
call("GET", bpath + "/assignment/events")
call("GET", "/api/v1/specialist/me/bookings", user=specialist)
call("GET", "/api/v1/specialist/me/schedule", user=specialist)
call("GET", "/api/v1/specialist/me/assignment-events", user=specialist)
clock("2027-01-02T11:00:00Z")
call("POST", "/api/v1/specialist/me/bookings/" + b["id"] + "/complete", user=specialist)
clock("2027-02-02T11:00:00Z")
call("POST", "/api/v1/specialist/me/bookings/" + b["id"] + "/undo-completion", {"note": "correction"}, specialist)
clock("2027-01-01T00:00:00Z")
for day, action in [(3, "approve"), (4, "reject"), (5, "withdraw")]:
    leave = call("POST", "/api/v1/specialist/me/absence-requests", {"kind": "day_off", "scope": "full_days", "startDate": f"2027-01-{day:02}", "endDate": f"2027-01-{day:02}"}, specialist)
    if action == "withdraw":
        call("POST", "/api/v1/specialist/me/absence-requests/" + leave["id"] + "/withdraw", user=specialist)
    else:
        lp = "/api/v1/admin/absence-requests/" + leave["id"]
        preview = call("POST", lp + "/preview")
        call("POST", lp + "/" + action, {"expectedCapacityRevision": preview["capacityRevision"], "reason": "reviewed"})
call("GET", "/api/v1/admin/absence-requests")
call("GET", "/api/v1/specialist/me/absence-requests", user=specialist)
room = call("POST", "/api/v1/admin/rooms", {"name": "Test Room", "sortOrder": 5})
call("GET", "/api/v1/admin/rooms")
call("PATCH", "/api/v1/admin/rooms/" + room["id"], {"name": "Updated Room", "isActive": False})
call("POST", bpath + "/cancel", {"note": "cancelled"})
call("POST", bpath + "/restore")
call("POST", bpath + "/cancel", {})
vp = call("POST", "/api/v1/admin/vacations", {"startDate": "2027-01-06", "endDate": "2027-01-06", "expectedRevision": mode_revision()})
call("PATCH", "/api/v1/admin/vacations/" + vp["id"], {"startDate": "2027-01-07", "endDate": "2027-01-07", "expectedRevision": mode_revision()})
call("GET", "/api/v1/admin/vacations")
call("DELETE", "/api/v1/admin/vacations/" + vp["id"], headers={"If-Match": str(mode_revision())})
simple = call("GET", "/api/v1/admin/schedule/simple")
simple["expectedRevision"] = simple.pop("revision")
call("PUT", "/api/v1/admin/schedule/simple", simple)
call("POST", "/api/v1/admin/schedule/mode/preview", {"mode": "advanced", "from": "2027-01-10", "to": "2027-01-13"})
call("PUT", "/api/v1/admin/schedule/mode", {"mode": "advanced", "expectedRevision": mode_revision()})
rule = {"kind": "working", "intervals": [{"start": "09:00", "end": "18:00"}], "stepOverride": None}
call("PUT", "/api/v1/admin/schedule/drafts/2027-01-10", {"rule": rule, "expectedRevision": mode_revision()})
call("PUT", "/api/v1/admin/schedule/drafts/range", {"from": "2027-01-11", "to": "2027-01-12", "rule": rule, "expectedRevision": mode_revision()})
call("POST", "/api/v1/admin/schedule/drafts/copy-simple", {"from": "2027-01-13", "to": "2027-01-13", "expectedRevision": mode_revision()})
call("GET", "/api/v1/admin/schedule/days?from=2027-01-10&to=2027-01-13&stage=draft")
rng = {"from": "2027-01-10", "to": "2027-01-13", "expectedRevision": mode_revision()}
call("POST", "/api/v1/admin/schedule/publications/preview", rng)
call("POST", "/api/v1/admin/schedule/publications", rng)
call("GET", "/api/v1/admin/schedule/days/2027-01-10")
call("GET", "/api/v1/admin/schedule/warnings")
history = call("GET", "/api/v1/admin/schedule/history")
hp = "/api/v1/admin/schedule/history/" + history["items"][0]["id"]
call("POST", hp + "/preview")
call("POST", hp + "/restore", {"expectedRevision": mode_revision()})
call("POST", "/api/v1/admin/exports")
call("DELETE", "/api/v1/auth/sessions/current", user=specialist)
call("POST", sp + "/archive")
expected = {row[3]["operationId"] for row in operations}
assert covered == expected, f"Unexercised operations: {sorted(expected - covered)}"
print(f"OpenAPI: {len(covered)}/79 operations exercised; all request/response schemas validated.")
