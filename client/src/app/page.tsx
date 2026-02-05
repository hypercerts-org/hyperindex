"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { graphqlClient } from "@/lib/graphql/client";
import {
  GET_STATISTICS,
  GET_ACTIVITY_BUCKETS,
  GET_RECENT_ACTIVITY,
  GET_SETTINGS,
} from "@/lib/graphql/queries";
import { StatsCards } from "@/components/dashboard/StatsCards";
import { ActivityChart } from "@/components/dashboard/ActivityChart";
import { RecentActivity } from "@/components/dashboard/RecentActivity";
import { Alert, Button } from "@/components/ui";
import Link from "next/link";
import type {
  StatisticsResponse,
  ActivityBucketsResponse,
  RecentActivityResponse,
  SettingsResponse,
  TimeRange,
} from "@/types";

export default function Dashboard() {
  const [timeRange, setTimeRange] = useState<TimeRange>("ONE_DAY");

  // Fetch statistics
  const { data: statsData, isLoading: statsLoading } = useQuery({
    queryKey: ["statistics"],
    queryFn: () => graphqlClient.request<StatisticsResponse>(GET_STATISTICS),
  });

  // Fetch settings for configuration alerts
  const { data: settingsData } = useQuery({
    queryKey: ["settings"],
    queryFn: () => graphqlClient.request<SettingsResponse>(GET_SETTINGS),
  });

  // Fetch activity buckets
  const { data: activityData, isLoading: activityLoading } = useQuery({
    queryKey: ["activityBuckets", timeRange],
    queryFn: () =>
      graphqlClient.request<ActivityBucketsResponse>(GET_ACTIVITY_BUCKETS, {
        range: timeRange,
      }),
  });

  // Fetch recent activity
  const { data: recentData, isLoading: recentLoading } = useQuery({
    queryKey: ["recentActivity"],
    queryFn: () =>
      graphqlClient.request<RecentActivityResponse>(GET_RECENT_ACTIVITY, {
        hours: 24,
      }),
  });

  const stats = statsData?.statistics ?? { recordCount: 0, actorCount: 0, lexiconCount: 0 };
  const settings = settingsData?.settings;

  return (
    <div className="space-y-8">
      {/* Configuration Alerts */}
      {settings && !settings.domainAuthority && (
        <Alert variant="warning">
          No domain authority configured.{" "}
          <Link href="/settings" className="font-medium underline">
            Configure in Settings
          </Link>
        </Alert>
      )}
      {stats.lexiconCount === 0 && (
        <Alert variant="info">
          No lexicons loaded.{" "}
          <Link href="/settings" className="font-medium underline">
            Upload lexicons in Settings
          </Link>
        </Alert>
      )}

      {/* Quick Actions */}
      <div className="flex flex-wrap gap-3">
        <Button asChild>
          <a href="/graphiql" target="_blank" rel="noopener noreferrer">
            Open GraphiQL
          </a>
        </Button>
        <Button variant="outline" asChild>
          <Link href="/backfill">Trigger Backfill</Link>
        </Button>
      </div>

      {/* Stats Cards */}
      <StatsCards
        recordCount={stats.recordCount}
        actorCount={stats.actorCount}
        lexiconCount={stats.lexiconCount}
        isLoading={statsLoading}
      />

      {/* Activity Chart */}
      <ActivityChart
        data={activityData?.activityBuckets ?? []}
        timeRange={timeRange}
        onTimeRangeChange={setTimeRange}
        isLoading={activityLoading}
      />

      {/* Recent Activity */}
      <RecentActivity
        entries={recentData?.recentActivity ?? []}
        isLoading={recentLoading}
      />
    </div>
  );
}
