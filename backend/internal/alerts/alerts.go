// Package alerts evaluates alert policies and routes escalations.
//
// Status: PHASE 4 STUB — Not yet implemented.
//
// When implemented, this package will:
//   - Evaluate alert_policies against health engine assessments
//   - Trigger Slack webhook notifications (Phase 4)
//   - Trigger PagerDuty routing key escalations (Phase 4)
//   - Support alert suppression windows and acknowledgement
//
// IMPORTANT: All notification credentials (SLACK_WEBHOOK_URL, PAGERDUTY_ROUTING_KEY)
// must be stored in a secrets manager. They are NEVER logged or stored in the database.
package alerts
