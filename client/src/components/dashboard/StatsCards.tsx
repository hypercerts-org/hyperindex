"use client";

import { Card, CardContent } from "@/components/ui";
import { formatNumber } from "@/lib/utils";
import { Database, Users, FileJson } from "lucide-react";

interface StatsCardsProps {
  recordCount: number;
  actorCount: number;
  lexiconCount: number;
  isLoading?: boolean;
}

export function StatsCards({
  recordCount,
  actorCount,
  lexiconCount,
  isLoading,
}: StatsCardsProps) {
  const stats = [
    {
      name: "Records",
      value: recordCount,
      icon: Database,
      color: "text-blue-600 dark:text-blue-400",
      bg: "bg-blue-100 dark:bg-blue-900/30",
    },
    {
      name: "Actors",
      value: actorCount,
      icon: Users,
      color: "text-green-600 dark:text-green-400",
      bg: "bg-green-100 dark:bg-green-900/30",
    },
    {
      name: "Lexicons",
      value: lexiconCount,
      icon: FileJson,
      color: "text-purple-600 dark:text-purple-400",
      bg: "bg-purple-100 dark:bg-purple-900/30",
    },
  ];

  return (
    <div className="grid gap-4 sm:grid-cols-3">
      {stats.map((stat) => (
        <Card key={stat.name}>
          <CardContent className="p-6">
            <div className="flex items-center gap-4">
              <div className={`rounded-lg p-3 ${stat.bg}`}>
                <stat.icon className={`h-6 w-6 ${stat.color}`} />
              </div>
              <div>
                <p className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
                  {stat.name}
                </p>
                {isLoading ? (
                  <div className="h-8 w-20 animate-pulse rounded bg-zinc-200 dark:bg-zinc-700" />
                ) : (
                  <p className="text-2xl font-bold text-zinc-900 dark:text-white">
                    {formatNumber(stat.value)}
                  </p>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
