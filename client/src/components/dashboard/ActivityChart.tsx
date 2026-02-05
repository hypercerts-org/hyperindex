"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui";
import { Button } from "@/components/ui";
import type { ActivityBucket, TimeRange } from "@/types";
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { format } from "date-fns";

interface ActivityChartProps {
  data: ActivityBucket[];
  timeRange: TimeRange;
  onTimeRangeChange: (range: TimeRange) => void;
  isLoading?: boolean;
}

const timeRanges: { value: TimeRange; label: string }[] = [
  { value: "ONE_HOUR", label: "1h" },
  { value: "THREE_HOURS", label: "3h" },
  { value: "SIX_HOURS", label: "6h" },
  { value: "ONE_DAY", label: "24h" },
  { value: "SEVEN_DAYS", label: "7d" },
];

export function ActivityChart({
  data,
  timeRange,
  onTimeRangeChange,
  isLoading,
}: ActivityChartProps) {
  const chartData = data.map((bucket) => ({
    timestamp: bucket.timestamp,
    creates: bucket.creates,
    updates: bucket.updates,
    deletes: bucket.deletes,
    total: bucket.creates + bucket.updates + bucket.deletes,
  }));

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle>Activity</CardTitle>
        <div className="flex gap-1">
          {timeRanges.map((range) => (
            <Button
              key={range.value}
              variant={timeRange === range.value ? "default" : "ghost"}
              size="sm"
              onClick={() => onTimeRangeChange(range.value)}
            >
              {range.label}
            </Button>
          ))}
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="h-64 animate-pulse rounded bg-zinc-200 dark:bg-zinc-700" />
        ) : data.length === 0 ? (
          <div className="flex h-64 items-center justify-center text-zinc-500 dark:text-zinc-400">
            No activity data available
          </div>
        ) : (
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-zinc-200 dark:stroke-zinc-700" />
                <XAxis
                  dataKey="timestamp"
                  tickFormatter={(value) =>
                    format(new Date(value), timeRange === "SEVEN_DAYS" ? "MMM d" : "HH:mm")
                  }
                  className="text-xs"
                />
                <YAxis className="text-xs" />
                <Tooltip
                  contentStyle={{
                    backgroundColor: "var(--color-zinc-900)",
                    border: "1px solid var(--color-zinc-700)",
                    borderRadius: "0.5rem",
                  }}
                  labelFormatter={(value) =>
                    format(new Date(value), "MMM d, yyyy HH:mm")
                  }
                />
                <Area
                  type="monotone"
                  dataKey="creates"
                  stackId="1"
                  stroke="#22c55e"
                  fill="#22c55e"
                  fillOpacity={0.6}
                  name="Creates"
                />
                <Area
                  type="monotone"
                  dataKey="updates"
                  stackId="1"
                  stroke="#3b82f6"
                  fill="#3b82f6"
                  fillOpacity={0.6}
                  name="Updates"
                />
                <Area
                  type="monotone"
                  dataKey="deletes"
                  stackId="1"
                  stroke="#ef4444"
                  fill="#ef4444"
                  fillOpacity={0.6}
                  name="Deletes"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
