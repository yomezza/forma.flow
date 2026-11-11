"""Regenerate HTTP metadata from the pinned OpenAPI document (developer tool)."""
import json
from pathlib import Path
import subprocess
import yaml

root = Path(__file__).resolve().parents[1]
spec = yaml.safe_load((root / "api/openapi.yaml").read_text())
transport = root / "internal/transport/http"
(transport / "schemas.json").write_text(json.dumps(spec["components"]["schemas"], ensure_ascii=False, separators=(",", ":")))
rows = ["// Code generated from api/openapi.yaml; DO NOT EDIT.\npackage httptransport\n", "type route struct { Method,Path,Operation,Schema,Role string; Status int; Body bool }\nvar routes=[]route{\n"]
for path, methods in spec["paths"].items():
    for method, op in methods.items():
        body = op.get("requestBody", {})
        schema = body.get("content", {}).get("application/json", {}).get("schema", {}).get("$ref", "").rsplit("/", 1)[-1]
        role = op.get("x-required-role", "admin" if "/admin/" in path else "specialist" if "/specialist/" in path else "staff" if op.get("security") else "")
        statuses = [int(x) for x in op["responses"] if x.startswith("2")]
        status = 201 if 201 in statuses else min(statuses)
        rows.append("{" + ",".join(json.dumps(x) for x in [method.upper(), path, op["operationId"], schema, role]) + "," + str(status) + "," + str(bool(body)).lower() + "},\n")
rows.append("}\n")
(transport / "routes.go").write_text("".join(rows))
subprocess.run(["gofmt", "-w", str(transport / "routes.go")], check=True)
