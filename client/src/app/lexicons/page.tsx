"use client";

import { useState, useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { FileText, Upload, ChevronDown, ChevronRight, Search, Trash2 } from "lucide-react";
import { graphqlClient } from "@/lib/graphql/client";
import { GET_LEXICONS } from "@/lib/graphql/queries";
import { UPLOAD_LEXICONS } from "@/lib/graphql/mutations";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";
import type { LexiconsResponse, Lexicon } from "@/types";

// JSON Syntax Highlighter Component
function JsonHighlight({ json }: { json: string }) {
  const highlighted = useMemo(() => {
    try {
      const parsed = JSON.parse(json);
      const formatted = JSON.stringify(parsed, null, 2);
      
      // Simple syntax highlighting
      return formatted
        .replace(/"([^"]+)":/g, '<span class="text-purple-400">"$1"</span>:')
        .replace(/: "([^"]+)"/g, ': <span class="text-green-400">"$1"</span>')
        .replace(/: (\d+)/g, ': <span class="text-amber-400">$1</span>')
        .replace(/: (true|false)/g, ': <span class="text-blue-400">$1</span>')
        .replace(/: (null)/g, ': <span class="text-gray-500">$1</span>');
    } catch {
      return json;
    }
  }, [json]);

  return (
    <pre
      className="text-xs overflow-x-auto bg-gray-950 p-4 rounded-lg text-gray-300 font-mono"
      dangerouslySetInnerHTML={{ __html: highlighted }}
    />
  );
}

// Single Lexicon Item Component
function LexiconItem({ lexicon }: { lexicon: Lexicon }) {
  const [expanded, setExpanded] = useState(false);
  
  // Parse lexicon to get description
  const parsed = useMemo(() => {
    try {
      return JSON.parse(lexicon.json);
    } catch {
      return null;
    }
  }, [lexicon.json]);

  const description = parsed?.defs?.main?.description || parsed?.description || "No description";
  const type = parsed?.defs?.main?.type || "unknown";

  return (
    <div className="border border-gray-800 rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between p-4 bg-gray-900 hover:bg-gray-800 transition-colors text-left"
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="h-4 w-4 text-gray-500" />
          ) : (
            <ChevronRight className="h-4 w-4 text-gray-500" />
          )}
          <FileText className="h-5 w-5 text-purple-400" />
          <div>
            <div className="font-mono text-sm text-white">{lexicon.id}</div>
            <div className="text-xs text-gray-500 mt-0.5">
              {type} - {description.slice(0, 80)}{description.length > 80 ? "..." : ""}
            </div>
          </div>
        </div>
        <div className="text-xs text-gray-500">
          {new Date(lexicon.createdAt).toLocaleDateString()}
        </div>
      </button>
      
      {expanded && (
        <div className="border-t border-gray-800">
          <JsonHighlight json={lexicon.json} />
        </div>
      )}
    </div>
  );
}

export default function LexiconsPage() {
  const queryClient = useQueryClient();
  const [searchQuery, setSearchQuery] = useState("");
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploadSuccess, setUploadSuccess] = useState(false);

  // Fetch lexicons
  const { data, isLoading, error } = useQuery({
    queryKey: ["lexicons"],
    queryFn: () => graphqlClient.request<LexiconsResponse>(GET_LEXICONS),
  });

  // Upload lexicons mutation
  const uploadMutation = useMutation({
    mutationFn: (zipBase64: string) =>
      graphqlClient.request(UPLOAD_LEXICONS, { zipBase64 }),
    onSuccess: () => {
      setUploadSuccess(true);
      setUploadError(null);
      queryClient.invalidateQueries({ queryKey: ["lexicons"] });
      setTimeout(() => setUploadSuccess(false), 3000);
    },
    onError: (err: Error) => {
      setUploadError(err.message);
      setUploadSuccess(false);
    },
  });

  // Handle file upload
  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.name.endsWith(".zip")) {
      setUploadError("Please upload a .zip file containing lexicon JSON files");
      return;
    }

    const reader = new FileReader();
    reader.onload = () => {
      const base64 = (reader.result as string).split(",")[1];
      uploadMutation.mutate(base64);
    };
    reader.onerror = () => {
      setUploadError("Failed to read file");
    };
    reader.readAsDataURL(file);
    
    // Reset input
    e.target.value = "";
  };

  // Filter lexicons by search query
  const filteredLexicons = useMemo(() => {
    if (!data?.lexicons) return [];
    if (!searchQuery) return data.lexicons;
    
    const query = searchQuery.toLowerCase();
    return data.lexicons.filter(
      (lex) =>
        lex.id.toLowerCase().includes(query) ||
        lex.json.toLowerCase().includes(query)
    );
  }, [data?.lexicons, searchQuery]);

  if (error) {
    return (
      <div className="p-6">
        <Alert variant="error">Failed to load lexicons: {(error as Error).message}</Alert>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Lexicons</h1>
          <p className="text-gray-400 mt-1">
            Manage AT Protocol lexicon definitions for your AppView
          </p>
        </div>
        <div>
          <input
            type="file"
            id="lexicon-upload"
            className="hidden"
            accept=".zip"
            onChange={handleFileUpload}
          />
          <label htmlFor="lexicon-upload">
            <Button
              as="span"
              disabled={uploadMutation.isPending}
              className="cursor-pointer"
            >
              <Upload className="h-4 w-4 mr-2" />
              {uploadMutation.isPending ? "Uploading..." : "Upload Lexicons"}
            </Button>
          </label>
        </div>
      </div>

      {/* Alerts */}
      {uploadError && (
        <Alert variant="error" onClose={() => setUploadError(null)}>
          {uploadError}
        </Alert>
      )}
      {uploadSuccess && (
        <Alert variant="success">Lexicons uploaded successfully!</Alert>
      )}

      {/* Search */}
      <Card>
        <CardContent className="p-4">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-500" />
            <Input
              placeholder="Search lexicons by ID or content..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-10"
            />
          </div>
        </CardContent>
      </Card>

      {/* Lexicon List */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <FileText className="h-5 w-5" />
            Installed Lexicons
            {data?.lexicons && (
              <span className="text-sm font-normal text-gray-500">
                ({filteredLexicons.length} of {data.lexicons.length})
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="text-center py-8 text-gray-500">Loading lexicons...</div>
          ) : filteredLexicons.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              {searchQuery
                ? "No lexicons match your search"
                : "No lexicons installed. Upload a ZIP file to get started."}
            </div>
          ) : (
            <div className="space-y-2">
              {filteredLexicons.map((lexicon) => (
                <LexiconItem key={lexicon.id} lexicon={lexicon} />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
