"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { RefreshCw, User, Play, Loader2, CheckCircle2, AlertCircle } from "lucide-react";
import { graphqlClient } from "@/lib/graphql/client";
import { IS_BACKFILLING } from "@/lib/graphql/queries";
import { TRIGGER_BACKFILL, BACKFILL_ACTOR } from "@/lib/graphql/mutations";
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import type { IsBackfillingResponse } from "@/types";

export default function BackfillPage() {
  const queryClient = useQueryClient();
  const [actorDid, setActorDid] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Check if backfill is running
  const { data: backfillStatus, isLoading: statusLoading } = useQuery({
    queryKey: ["backfillStatus"],
    queryFn: () => graphqlClient.request<IsBackfillingResponse>(IS_BACKFILLING),
    refetchInterval: 5000, // Poll every 5 seconds
  });

  // Trigger full backfill mutation
  const triggerBackfillMutation = useMutation({
    mutationFn: () => graphqlClient.request(TRIGGER_BACKFILL),
    onSuccess: () => {
      setSuccess("Full backfill started successfully");
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["backfillStatus"] });
      setTimeout(() => setSuccess(null), 5000);
    },
    onError: (err: Error) => {
      setError(err.message);
      setSuccess(null);
    },
  });

  // Backfill single actor mutation
  const backfillActorMutation = useMutation({
    mutationFn: (did: string) => graphqlClient.request(BACKFILL_ACTOR, { did }),
    onSuccess: () => {
      setSuccess(`Backfill started for ${actorDid}`);
      setError(null);
      setActorDid("");
      queryClient.invalidateQueries({ queryKey: ["backfillStatus"] });
      setTimeout(() => setSuccess(null), 5000);
    },
    onError: (err: Error) => {
      setError(err.message);
      setSuccess(null);
    },
  });

  const handleActorBackfill = (e: React.FormEvent) => {
    e.preventDefault();
    if (!actorDid.trim()) {
      setError("Please enter a DID");
      return;
    }
    if (!actorDid.startsWith("did:")) {
      setError("Invalid DID format. DIDs should start with 'did:'");
      return;
    }
    backfillActorMutation.mutate(actorDid.trim());
  };

  const isBackfilling = backfillStatus?.isBackfilling ?? false;

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-white">Backfill</h1>
        <p className="text-gray-400 mt-1">
          Sync historical data from the AT Protocol relay
        </p>
      </div>

      {/* Alerts */}
      {error && (
        <Alert variant="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      {success && (
        <Alert variant="success" onClose={() => setSuccess(null)}>
          {success}
        </Alert>
      )}

      {/* Status Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <RefreshCw className={`h-5 w-5 ${isBackfilling ? "animate-spin" : ""}`} />
            Backfill Status
          </CardTitle>
          <CardDescription>
            Current status of the background backfill process
          </CardDescription>
        </CardHeader>
        <CardContent>
          {statusLoading ? (
            <div className="flex items-center gap-2 text-gray-500">
              <Loader2 className="h-4 w-4 animate-spin" />
              Checking status...
            </div>
          ) : isBackfilling ? (
            <div className="flex items-center gap-3 p-4 bg-blue-500/10 border border-blue-500/20 rounded-lg">
              <Loader2 className="h-5 w-5 animate-spin text-blue-400" />
              <div>
                <div className="font-medium text-blue-400">Backfill in progress</div>
                <div className="text-sm text-gray-400">
                  Syncing records from the relay. This may take a while.
                </div>
              </div>
            </div>
          ) : (
            <div className="flex items-center gap-3 p-4 bg-gray-800 rounded-lg">
              <CheckCircle2 className="h-5 w-5 text-green-400" />
              <div>
                <div className="font-medium text-white">Idle</div>
                <div className="text-sm text-gray-400">
                  No backfill currently running
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Full Backfill Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <RefreshCw className="h-5 w-5" />
            Full Backfill
          </CardTitle>
          <CardDescription>
            Trigger a complete backfill of all known actors from the relay.
            This will fetch all historical records for actors that have been seen.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-4">
            <Button
              onClick={() => triggerBackfillMutation.mutate()}
              disabled={triggerBackfillMutation.isPending || isBackfilling}
              variant={isBackfilling ? "secondary" : "primary"}
            >
              {triggerBackfillMutation.isPending ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Starting...
                </>
              ) : isBackfilling ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Backfill Running
                </>
              ) : (
                <>
                  <Play className="h-4 w-4 mr-2" />
                  Start Full Backfill
                </>
              )}
            </Button>
            {isBackfilling && (
              <span className="text-sm text-gray-500">
                A backfill is already in progress
              </span>
            )}
          </div>

          <div className="mt-4 p-4 bg-amber-500/10 border border-amber-500/20 rounded-lg">
            <div className="flex items-start gap-2">
              <AlertCircle className="h-4 w-4 text-amber-400 mt-0.5" />
              <div className="text-sm text-amber-200">
                <strong>Note:</strong> Full backfill can take a long time depending on the number
                of actors and records. The process runs in the background and will continue
                even if you navigate away from this page.
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Actor Backfill Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <User className="h-5 w-5" />
            Backfill Single Actor
          </CardTitle>
          <CardDescription>
            Fetch all historical records for a specific actor by their DID.
            Useful for quickly syncing a single user.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleActorBackfill} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">
                Actor DID
              </label>
              <Input
                type="text"
                placeholder="did:plc:..."
                value={actorDid}
                onChange={(e) => setActorDid(e.target.value)}
                className="font-mono"
              />
              <p className="text-xs text-gray-500 mt-1">
                Enter the full DID of the actor (e.g., did:plc:z72i7hdynmk6r22z27h6tvur)
              </p>
            </div>
            <Button
              type="submit"
              disabled={backfillActorMutation.isPending || !actorDid.trim()}
            >
              {backfillActorMutation.isPending ? (
                <>
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  Starting...
                </>
              ) : (
                <>
                  <Play className="h-4 w-4 mr-2" />
                  Backfill Actor
                </>
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
