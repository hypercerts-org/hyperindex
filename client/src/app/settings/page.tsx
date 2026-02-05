"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { graphqlClient } from "@/lib/graphql/client";
import { GET_SETTINGS, GET_OAUTH_CLIENTS } from "@/lib/graphql/queries";
import { UPDATE_SETTINGS, RESET_ALL, UPLOAD_LEXICONS } from "@/lib/graphql/mutations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
  Button,
  Input,
  Alert,
} from "@/components/ui";
import type { SettingsResponse, OAuthClientsResponse } from "@/types";

export default function SettingsPage() {
  const queryClient = useQueryClient();

  // Fetch settings
  const { data: settingsData, isLoading } = useQuery({
    queryKey: ["settings"],
    queryFn: () => graphqlClient.request<SettingsResponse>(GET_SETTINGS),
  });

  // Fetch OAuth clients
  const { data: oauthData } = useQuery({
    queryKey: ["oauthClients"],
    queryFn: () => graphqlClient.request<OAuthClientsResponse>(GET_OAUTH_CLIENTS),
  });

  const settings = settingsData?.settings;
  const oauthClients = oauthData?.oauthClients ?? [];

  // Form state
  const [domainAuthority, setDomainAuthority] = useState("");
  const [relayUrl, setRelayUrl] = useState("");
  const [plcDirectoryUrl, setPlcDirectoryUrl] = useState("");
  const [jetstreamUrl, setJetstreamUrl] = useState("");
  const [oauthScopes, setOauthScopes] = useState("");
  const [resetConfirmation, setResetConfirmation] = useState("");
  const [alert, setAlert] = useState<{ type: "success" | "error"; message: string } | null>(null);

  // Update form when settings load
  useState(() => {
    if (settings) {
      setDomainAuthority(settings.domainAuthority);
      setRelayUrl(settings.relayUrl);
      setPlcDirectoryUrl(settings.plcDirectoryUrl);
      setJetstreamUrl(settings.jetstreamUrl);
      setOauthScopes(settings.oauthSupportedScopes);
    }
  });

  // Update settings mutation
  const updateMutation = useMutation({
    mutationFn: (variables: Record<string, unknown>) =>
      graphqlClient.request(UPDATE_SETTINGS, variables),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
      setAlert({ type: "success", message: "Settings updated successfully" });
    },
    onError: (error: Error) => {
      setAlert({ type: "error", message: error.message });
    },
  });

  // Reset mutation
  const resetMutation = useMutation({
    mutationFn: (confirm: string) =>
      graphqlClient.request(RESET_ALL, { confirm }),
    onSuccess: () => {
      queryClient.invalidateQueries();
      setResetConfirmation("");
      setAlert({ type: "success", message: "All data has been reset" });
    },
    onError: (error: Error) => {
      setAlert({ type: "error", message: error.message });
    },
  });

  const handleSaveSettings = () => {
    updateMutation.mutate({
      domainAuthority: domainAuthority || undefined,
      relayUrl: relayUrl || undefined,
      plcDirectoryUrl: plcDirectoryUrl || undefined,
      jetstreamUrl: jetstreamUrl || undefined,
      oauthSupportedScopes: oauthScopes || undefined,
      adminDids: settings?.adminDids,
    });
  };

  const handleReset = () => {
    if (resetConfirmation === "RESET") {
      resetMutation.mutate("RESET");
    }
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        {[...Array(3)].map((_, i) => (
          <div key={i} className="h-48 animate-pulse rounded-lg bg-zinc-200 dark:bg-zinc-800" />
        ))}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-zinc-900 dark:text-white">Settings</h1>
        <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
          Configure your Hypergoat instance
        </p>
      </div>

      {alert && (
        <Alert variant={alert.type === "success" ? "success" : "error"}>
          {alert.message}
        </Alert>
      )}

      {/* Basic Settings */}
      <Card>
        <CardHeader>
          <CardTitle>Basic Settings</CardTitle>
          <CardDescription>Configure the core settings for your instance</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Input
            label="Domain Authority"
            placeholder="your-domain.com"
            value={domainAuthority}
            onChange={(e) => setDomainAuthority(e.target.value)}
          />
          <div className="flex justify-end">
            <Button
              onClick={handleSaveSettings}
              loading={updateMutation.isPending}
            >
              Save Settings
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* External Services */}
      <Card>
        <CardHeader>
          <CardTitle>External Services</CardTitle>
          <CardDescription>Configure external service URLs</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Input
            label="Relay URL"
            placeholder="https://relay1.us-west.bsky.network"
            value={relayUrl}
            onChange={(e) => setRelayUrl(e.target.value)}
          />
          <Input
            label="PLC Directory URL"
            placeholder="https://plc.directory"
            value={plcDirectoryUrl}
            onChange={(e) => setPlcDirectoryUrl(e.target.value)}
          />
          <Input
            label="Jetstream URL"
            placeholder="wss://jetstream2.us-west.bsky.network/subscribe"
            value={jetstreamUrl}
            onChange={(e) => setJetstreamUrl(e.target.value)}
          />
          <Input
            label="OAuth Supported Scopes"
            placeholder="atproto transition:generic"
            value={oauthScopes}
            onChange={(e) => setOauthScopes(e.target.value)}
          />
          <div className="flex justify-end">
            <Button
              onClick={handleSaveSettings}
              loading={updateMutation.isPending}
            >
              Save Settings
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Admin DIDs */}
      <Card>
        <CardHeader>
          <CardTitle>Administrators</CardTitle>
          <CardDescription>Manage admin access to this instance</CardDescription>
        </CardHeader>
        <CardContent>
          {settings?.adminDids.length === 0 ? (
            <p className="text-sm text-zinc-500 dark:text-zinc-400">
              No administrators configured
            </p>
          ) : (
            <ul className="divide-y divide-zinc-200 dark:divide-zinc-800">
              {settings?.adminDids.map((did) => (
                <li key={did} className="flex items-center justify-between py-3">
                  <code className="text-sm">{did}</code>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {/* OAuth Clients */}
      <Card>
        <CardHeader>
          <CardTitle>OAuth Clients</CardTitle>
          <CardDescription>Manage registered OAuth clients</CardDescription>
        </CardHeader>
        <CardContent>
          {oauthClients.length === 0 ? (
            <p className="text-sm text-zinc-500 dark:text-zinc-400">
              No OAuth clients registered
            </p>
          ) : (
            <ul className="divide-y divide-zinc-200 dark:divide-zinc-800">
              {oauthClients.map((client) => (
                <li key={client.clientId} className="py-3">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="font-medium text-zinc-900 dark:text-white">
                        {client.clientName}
                      </p>
                      <code className="text-xs text-zinc-500">{client.clientId}</code>
                    </div>
                    <span className="rounded-full bg-zinc-100 px-2 py-1 text-xs dark:bg-zinc-800">
                      {client.clientType}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {/* Danger Zone */}
      <Card className="border-red-200 dark:border-red-900">
        <CardHeader>
          <CardTitle className="text-red-600">Danger Zone</CardTitle>
          <CardDescription>Destructive actions - proceed with caution</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            Reset all data including records, actors, and activity. This action cannot be undone.
          </p>
          <div className="flex items-end gap-4">
            <Input
              label="Type RESET to confirm"
              placeholder="RESET"
              value={resetConfirmation}
              onChange={(e) => setResetConfirmation(e.target.value)}
              className="max-w-xs"
            />
            <Button
              variant="destructive"
              onClick={handleReset}
              disabled={resetConfirmation !== "RESET"}
              loading={resetMutation.isPending}
            >
              Reset All Data
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
