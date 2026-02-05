"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui";
import { formatRelative } from "@/lib/utils";
import { cn } from "@/lib/utils";
import type { ActivityEntry } from "@/types";
import { CheckCircle, XCircle, AlertCircle } from "lucide-react";

interface RecentActivityProps {
  entries: ActivityEntry[];
  isLoading?: boolean;
}

export function RecentActivity({ entries, isLoading }: RecentActivityProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent Activity</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-3">
            {[...Array(5)].map((_, i) => (
              <div
                key={i}
                className="h-12 animate-pulse rounded bg-zinc-200 dark:bg-zinc-700"
              />
            ))}
          </div>
        ) : entries.length === 0 ? (
          <div className="py-8 text-center text-zinc-500 dark:text-zinc-400">
            No recent activity
          </div>
        ) : (
          <div className="divide-y divide-zinc-200 dark:divide-zinc-800">
            {entries.slice(0, 10).map((entry) => (
              <div
                key={entry.id}
                className="flex items-center justify-between py-3"
              >
                <div className="flex items-center gap-3">
                  <StatusIcon status={entry.status} />
                  <div>
                    <div className="flex items-center gap-2">
                      <span
                        className={cn(
                          "rounded px-2 py-0.5 text-xs font-medium",
                          entry.operation === "create" &&
                            "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
                          entry.operation === "update" &&
                            "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
                          entry.operation === "delete" &&
                            "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                        )}
                      >
                        {entry.operation}
                      </span>
                      <span className="text-sm font-medium text-zinc-900 dark:text-white">
                        {entry.collection}
                      </span>
                    </div>
                    <p className="text-xs text-zinc-500 dark:text-zinc-400">
                      {entry.did}
                    </p>
                  </div>
                </div>
                <div className="text-right">
                  <p className="text-xs text-zinc-500 dark:text-zinc-400">
                    {formatRelative(entry.timestamp)}
                  </p>
                  {entry.errorMessage && (
                    <p className="text-xs text-red-500">{entry.errorMessage}</p>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function StatusIcon({ status }: { status: string }) {
  if (status === "success") {
    return <CheckCircle className="h-5 w-5 text-green-500" />;
  }
  if (status === "error") {
    return <XCircle className="h-5 w-5 text-red-500" />;
  }
  return <AlertCircle className="h-5 w-5 text-yellow-500" />;
}
