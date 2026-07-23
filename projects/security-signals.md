# Datadog Security Signals

Status: Implemented and live-tested
Last Updated: 2026-07-23
Branch: `feat/security-signals`
Worktree: `/Users/jsierles/bz/datadog-cli-security-signals`

## Progress

- [x] Add read-only security signal search/get commands.
- [x] Document `security_monitoring_signals_read` and usage.
- [x] Test against the reported ECS registration signal.
- [x] Confirm the signal is a real anomaly across three OpenResty instances, not a stale re-notification.

## Next Steps

1. Review and merge the feature branch.
2. Publish a release so Zooglerails `bin/dd` downloads the new commands.
3. Verify the ECS-agent bootstrap fix is applied to every non-ECS AMI consumer.

## Related Resources

- Conversation ID: `fc6de74b-840e-4af9-9e57-d318e37d7b01`
