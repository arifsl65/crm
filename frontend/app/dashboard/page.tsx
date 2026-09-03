'use client';

// The dashboard content is now rendered by DashboardShell based on the activePanel state.
// The panels (TodayPanel, OverviewPanel, DeadlinesPanel, etc.) are rendered automatically
// based on the submenu selection. This page component is kept for Next.js routing purposes
// but doesn't need to render anything itself since DashboardShell handles the panel rendering.

export default function DashboardPage() {
  // The actual panel content is rendered by DashboardShell.renderDashboardPanel()
  // This component doesn't need to render anything specific.
  return null;
}
