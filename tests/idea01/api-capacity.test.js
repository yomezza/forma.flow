const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { webcrypto } = require('node:crypto');

function createApi() {
  const storage = new Map();
  const context = {
    console,
    Date,
    Intl,
    Math,
    JSON,
    Map,
    Set,
    Object,
    Array,
    String,
    Number,
    RegExp,
    Error,
    Promise,
    crypto: webcrypto,
    Event,
    APP_CONFIG: { preloaderDuration: 0, demoMode: true },
    localStorage: {
      getItem: key => storage.has(key) ? storage.get(key) : null,
      setItem: (key, value) => storage.set(key, value),
      removeItem: key => storage.delete(key)
    },
    window: { addEventListener() {}, dispatchEvent() {} },
    navigator: {}
  };

  vm.createContext(context);
  for (const file of ['../../dist/js/demo-data.js', '../../dist/js/assignment-engine.js']) {
    vm.runInContext(fs.readFileSync(path.join(__dirname, file), 'utf8'), context);
  }
  const apiSource = fs.readFileSync(path.join(__dirname, '../../dist/js/demo-api.js'), 'utf8');
  vm.runInContext(apiSource + ';globalThis.testAPI=DemoAPI;', context);
  return context.testAPI;
}

async function run() {
  const migrationApi = createApi();
  const legacyBooking = await migrationApi.createBookingRequest({ serviceId: 'classic', date: '2026-12-03', time: '09:00', fullName: 'Клиент Старый', phone: '+79993333333' });
  await migrationApi.createClient({ fullName: 'Анна Общая', phone: '+79994444444' });
  await migrationApi.createClient({ fullName: 'Иван Общий', phone: '+79994444444' });
  const legacyV3 = structuredClone(await migrationApi.exportData());
  legacyV3.version = 3;
  for (const field of ['revision', 'specialists', 'rooms', 'specialistServices', 'specialistSchedules', 'absenceRequests', 'accounts', 'demoSession', 'assignmentEvents', 'auditEvents', 'capacityExceptions']) delete legacyV3[field];
  for (const booking of legacyV3.bookings) for (const field of ['specialistId', 'roomId', 'assignmentUpdatedAt', 'completion']) delete booking[field];
  await migrationApi.importData(legacyV3);
  const migrated = await migrationApi.exportData();
  assert.equal(migrated.version, 4);
  assert.equal(migrated.bookings[0].id, legacyBooking.id);
  assert.equal(migrated.bookings[0].specialistId, null);
  assert.equal(migrated.bookings[0].roomId, null);
  assert.equal(migrated.clients.filter(client => client.phoneNormalized === '+79994444444').length, 2);
  assert.equal(migrated.specialists.filter(item => item.state === 'active').length, 1);
  assert.equal(migrated.rooms.filter(item => item.isActive).length, 1);
  await migrationApi.importData(migrated);
  const repeatedMigration = await migrationApi.exportData();
  assert.equal(repeatedMigration.specialists.length, 1);
  assert.equal(repeatedMigration.rooms.length, 1);
  const stableBeforeInvalidImport = await migrationApi.exportData();
  await assert.rejects(migrationApi.importData({ ...stableBeforeInvalidImport, version: 5 }));
  assert.equal((await migrationApi.exportData()).revision, stableBeforeInvalidImport.revision);
  console.log('PASS v3 migration, repeated import, and duplicate phone preservation');

  const api = createApi();
  const ownAccount = await api.getMyDemoAccount();
  assert.deepEqual([...ownAccount.roles], ['admin', 'specialist']);
  assert.equal(ownAccount.activeRole, 'admin');
  await api.switchDemoRole('specialist');
  assert.equal((await api.getMyDemoAccount()).activeRole, 'specialist');
  await api.switchDemoRole('admin');
  const room = await api.createRoom({ name: 'Кабинет 2', sortOrder: 1 });
  const specialist = await api.createSpecialist({
    fullName: 'Специалист два',
    serviceIds: ['classic'],
    week: Object.fromEntries([0, 1, 2, 3, 4, 5, 6].map(day => [
      day,
      [{ start: '09:00', end: '21:00' }]
    ]))
  });

  const date = '2026-11-30';
  const time = '13:00';
  const first = await api.createBookingRequest({
    serviceId: 'classic',
    date,
    time,
    fullName: 'Клиент Первый',
    phone: '+79991111111'
  });
  const displaced = await api.createBookingRequest({
    serviceId: 'relax',
    date,
    time,
    fullName: 'Клиент Второй',
    phone: '+79992222222'
  });
  const candidates = await api.getAssignmentCandidates(first.id);
  assert.ok(Array.isArray(candidates.reasons));
  assert.ok(candidates.candidates.every(item => Array.isArray(item.roomIds) && !Object.hasOwn(item, 'roomId')));

  const preview = await api.previewBookingAssignment(first.id, {
    specialistId: 'sp-001',
    roomId: 'room-001'
  });
  assert.equal(preview.allowed, true);
  assert.equal(preview.requiresConfirmation, true);
  assert.deepEqual([...preview.affectedBookingIds], [displaced.id]);

  await assert.rejects(
    api.assignBooking(first.id, {
      specialistId: 'sp-001',
      roomId: 'room-001',
      expectedRevision: preview.revision
    }),
    error => error.code === 'CONFLICT_ACK_REQUIRED'
  );

  await api.assignBooking(first.id, {
    specialistId: 'sp-001',
    roomId: 'room-001',
    expectedRevision: preview.revision,
    acknowledgedBookingIds: [displaced.id]
  });

  const afterOverride = await api.exportData();
  assert.ok(afterOverride.capacityExceptions.some(exception =>
    exception.resolvedAt === null && exception.bookingIds.includes(displaced.id)
  ));
  assert.ok(!(await api.getAvailableSlots(date, 'classic')).includes(time));

  await api.setSpecialistServices(specialist.id, ['classic', 'relax']);
  const resolution = await api.previewBookingAssignment(displaced.id, {
    specialistId: specialist.id,
    roomId: room.id
  });
  assert.equal(resolution.allowed, true);
  await api.assignBooking(displaced.id, {
    specialistId: specialist.id,
    roomId: room.id,
    expectedRevision: resolution.revision
  });

  const finalData = await api.exportData();
  assert.ok(finalData.capacityExceptions.every(exception => exception.resolvedAt !== null));

  const privacyFixture = await api.exportData();
  const completed = privacyFixture.bookings[0];
  completed.status = 'completed';
  completed.specialistId = 'sp-001';
  completed.clientPhone = '+79990000000';
  completed.adminNote = 'admin-only';
  completed.completion = { bySpecialistId: 'sp-001', at: '2026-11-01T10:00:00.000Z' };
  await api.importData(privacyFixture);
  await api.selectDemoAccount('acct-owner');
  await api.switchDemoRole('specialist');
  const undoResult = await api.undoMyCompletion(completed.id, { reason: 'Проверка DTO' });
  const specialistBookings = await api.getMyBookings();
  for (const dto of [undoResult, specialistBookings[0]]) {
    assert.equal(Object.hasOwn(dto, 'clientPhone'), false);
    assert.equal(Object.hasOwn(dto, 'adminNote'), false);
    assert.equal(Object.hasOwn(dto, 'clientId'), false);
    assert.equal(Object.hasOwn(dto, 'completion'), false);
  }
  await assert.rejects(api.getSpecialistSchedule('sp-001'), error => error.code === 'ROLE_FORBIDDEN');
  await assert.rejects(api.getRoomsAndServicesForSpecialist('sp-001'), error => error.code === 'ROLE_FORBIDDEN');
  await api.switchDemoRole('admin');
  await api.setSpecialistServices(specialist.id, []);
  await api.switchDemoRole('specialist');
  const absenceDate = '2026-12-01';
  const unaffected = await api.getAvailableSlots(absenceDate, 'classic');
  assert.ok(unaffected.includes('15:00'));
  await api.submitMyAbsence({ kind: 'day_off', scope: 'partial_day', startDate: absenceDate, endDate: absenceDate, startTime: '14:00', endTime: '17:00', reason: 'Тест частичного отсутствия' });
  assert.ok((await api.getAvailableSlots(absenceDate, 'classic')).includes('15:00'));
  await api.switchDemoRole('admin');
  const absence = (await api.getAbsenceRequests({ status: 'pending' })).find(item => item.startDate === absenceDate);
  const absencePreview = await api.previewAbsenceApproval(absence.id);
  await api.approveAbsence(absence.id, { expectedRevision: absencePreview.revision, acknowledgedBookingIds: [] });
  assert.ok(!(await api.getAvailableSlots(absenceDate, 'classic')).includes('15:00'));

  const schedule = await api.getSpecialistSchedule('sp-001');
  assert.equal(schedule.dataRevision, (await api.exportData()).revision);
  await api.updateSpecialist('sp-001', { fullName: 'Специалист владелец' });
  await assert.rejects(api.setSpecialistSchedule('sp-001', { week: schedule.week, overrides: schedule.overrides, expectedRevision: schedule.dataRevision }), error => error.code === 'STALE_REVISION');
  const currentSchedule = await api.getSpecialistSchedule('sp-001');
  const overrideDate = '2026-12-08';
  await api.setSpecialistSchedule('sp-001', { week: currentSchedule.week, overrides: { ...currentSchedule.overrides, [overrideDate]: { kind: 'off', intervals: [] } }, expectedRevision: currentSchedule.dataRevision });
  assert.deepEqual([...(await api.getAvailableSlots(overrideDate, 'classic'))], []);
  const beforeInvalidSchedule = await api.exportData();
  const badWeek = structuredClone(schedule.week);
  badWeek[1] = [{ start: '12:00', end: '11:00' }];
  await assert.rejects(api.setSpecialistSchedule('sp-001', { week: badWeek, overrides: schedule.overrides, expectedRevision: beforeInvalidSchedule.revision }), error => error.code === 'INVALID_DATABASE');
  assert.equal((await api.exportData()).revision, beforeInvalidSchedule.revision);
  const cancelledFixture = await api.exportData();
  const cancelled = cancelledFixture.bookings.find(item => item.id === completed.id);
  cancelled.status = 'cancelled';
  cancelled.completion = null;
  await api.importData(cancelledFixture);
  await assert.rejects(api.previewBookingAssignment(cancelled.id, { specialistId: 'sp-001' }), error => error.code === 'INVALID_STATUS_TRANSITION');
  console.log('PASS partial absence, role guards, and schedule validation');
  console.log('PASS specialist mutation privacy DTO');
  console.log('PASS indirect assignment conflict lifecycle');
}

run().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
