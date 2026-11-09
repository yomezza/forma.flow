// Pure, deterministic joint assignment matcher for the FORMA TELA demo.
(() => {
  const minute = value => Number(value.slice(0, 2)) * 60 + Number(value.slice(3));
  const dayOfWeek = date => new Date(date + 'T12:00:00').getDay();
  const overlaps = (a, b) => a.start < b.end && b.start < a.end;

  function intervalsFor(db, specialistId, date) {
    const schedule = db.specialistSchedules.find(row => row.specialistId === specialistId);
    if (!schedule) return [];
    const override = schedule.overrides?.[date];
    if (override) return override.kind === 'working' ? override.intervals || [] : [];
    return schedule.week?.[dayOfWeek(date)] || [];
  }

  function solveAssignments(db, { date, candidate = null, fixedOverrides = {}, excludedIds = [], nodeLimit = 50000 } = {}) {
    const bookings = db.bookings.filter(row =>
      row.date === date && ['pending_confirmation', 'confirmed'].includes(row.status) &&
      !excludedIds.includes(row.id)
    );
    if (candidate) bookings.push({ ...candidate, id: candidate.id || '__candidate__' });
    const rooms = db.rooms.filter(room => room.isActive).sort((a, b) => a.sortOrder - b.sortOrder || a.id.localeCompare(b.id));
    const specialists = db.specialists.filter(row => row.state === 'active').sort((a, b) => a.id.localeCompare(b.id));
    const breakMinutes = db.settings.breakMinutes || 0;
    const domains = new Map();

    for (const booking of bookings) {
      const start = minute(booking.time);
      const end = start + booking.durationMinutes;
      const forced = fixedOverrides[booking.id] || {};
      const assignedSpecialist = forced.specialistId ?? booking.specialistId ?? null;
      const assignedRoom = forced.roomId ?? booking.roomId ?? null;
      const approvedAbsence = specialistId => db.absenceRequests.some(absence =>
        absence.specialistId === specialistId && absence.status === 'approved' &&
        date >= absence.startDate && date <= absence.endDate &&
        (absence.scope !== 'partial_day' || overlaps(
          { start, end: end + breakMinutes },
          { start: minute(absence.startTime), end: minute(absence.endTime) }
        ))
      );
      const allowedServices = new Set(db.specialistServices.filter(item => item.serviceId === booking.serviceId).map(item => item.specialistId));
      const pairs = [];
      for (const specialist of specialists) {
        if (assignedSpecialist && specialist.id !== assignedSpecialist) continue;
        if (!allowedServices.has(specialist.id) || approvedAbsence(specialist.id)) continue;
        const intervals = intervalsFor(db, specialist.id, date);
        if (!intervals.some(interval => start >= minute(interval.start) && end <= minute(interval.end))) continue;
        for (const room of rooms) {
          if (!assignedRoom || room.id === assignedRoom) pairs.push({ specialistId: specialist.id, roomId: room.id });
        }
      }
      domains.set(booking.id, pairs);
    }

    const occupiedSpecialists = new Map();
    const occupiedRooms = new Map();
    let examinedNodes = 0;
    const allocation = {};
    const ordered = [...bookings].sort((a, b) => (domains.get(a.id).length - domains.get(b.id).length) || a.id.localeCompare(b.id));
    function search(index) {
      if (index === ordered.length) return true;
      if (++examinedNodes > nodeLimit) return null;
      const booking = ordered[index];
      const start = minute(booking.time);
      const end = start + booking.durationMinutes + breakMinutes;
      for (const pair of domains.get(booking.id)) {
        const specialistRanges = occupiedSpecialists.get(pair.specialistId) || [];
        const roomRanges = occupiedRooms.get(pair.roomId) || [];
        if (specialistRanges.some(range => overlaps({ start, end }, range)) ||
            roomRanges.some(range => overlaps({ start, end }, range))) continue;
        specialistRanges.push({ start, end });
        roomRanges.push({ start, end });
        occupiedSpecialists.set(pair.specialistId, specialistRanges);
        occupiedRooms.set(pair.roomId, roomRanges);
        allocation[booking.id] = pair;
        const outcome = search(index + 1);
        if (outcome === true) return true;
        delete allocation[booking.id];
        specialistRanges.pop();
        roomRanges.pop();
        if (outcome === null) return null;
      }
      return false;
    }
    const outcome = search(0);
    const feasible = outcome === true;
    const uncertain = outcome === null;
    return {
      feasible,
      uncertain,
      status: uncertain ? 'UNKNOWN' : feasible ? 'SAT' : 'UNSAT',
      allocations: feasible ? allocation : {},
      reason: uncertain ? 'CAPACITY_CHECK_LIMIT' : feasible ? null : 'GLOBAL_CAPACITY_CONFLICT',
      examinedNodes,
      affectedBookingIds: feasible ? [] : bookings.filter(row => row.id !== candidate?.id).map(row => row.id)
    };
  }

  globalThis.FormaAssignmentEngine = Object.freeze({ solveAssignments });
})();
