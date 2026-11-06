# Implementation plan

See go-implementation-guide.md for phase-by-phase acceptance criteria. Suggested order: initial migrations + primary resource timezone; public catalogue/content; pure scheduling engine; booking transaction + concurrent tests; admin sessions/CSRF; status actions and client management; advanced schedule drafts/publish/history; frontend SiteAPI adapter; security, backups and production observability.

Do not implement a feature in Go merely because the DemoAPI method exists: importData, resetDemoData, clearBookings, clearClientsAndBookings and hard deleteBooking are demo maintenance and require a separate explicit administrative/data-retention policy.

All changes to main and frontend remain outside the architecture author's responsibility. Never merge without user approval.
