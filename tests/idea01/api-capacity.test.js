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
  const api = createApi();
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
  console.log('PASS indirect assignment conflict lifecycle');
}

run().catch(error => {
  console.error(error);
  process.exitCode = 1;
});
