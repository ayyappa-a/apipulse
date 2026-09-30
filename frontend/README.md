# APIPulse Frontend — SRE Command Center

The web interface for APIPulse, built with React 18, TypeScript, Vite, and custom CSS design tokens.

## Architecture

- **`src/app/`**: Application shell, layout root, and routing map.
- **`src/components/layout/`**: Collapsible Sidebar and Topbar with keyboard search (Cmd+K).
- **`src/components/ui/`**: StatusBadge, HealthDot, MetricCard, LatencyMetricRow.
- **`src/features/`**: Feature-scoped modules (probe waterfall, incident timeline, telemetry matrix).
- **`src/services/`**: Abstract service contracts (`IDependencyService`, `IProbeService`, etc.) with mock implementations.
- **`src/hooks/`**: Clean React data hooks (`useDependencies`, `useIncidents`, `useTelemetryStream`).
- **`src/types/`**: Domain models including `TelemetrySource` and data provenance.
- **`src/styles/`**: Global dark-mode tokens, elevation levels, and typography.
