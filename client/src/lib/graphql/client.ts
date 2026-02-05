import { GraphQLClient } from "graphql-request";

// Get the base URL for API requests
// - In browser: use current origin (Next.js rewrites proxy to backend)
// - On server (SSR): use the full API URL directly
function getBaseUrl(): string {
  if (typeof window !== "undefined") {
    return window.location.origin;
  }
  return process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
}

// Lazy-initialized clients to ensure proper URL detection after hydration
let _graphqlClient: GraphQLClient | null = null;
let _publicGraphqlClient: GraphQLClient | null = null;

export const graphqlClient = {
  request: <T>(document: string, variables?: Record<string, unknown>): Promise<T> => {
    if (!_graphqlClient) {
      _graphqlClient = new GraphQLClient(`${getBaseUrl()}/admin/graphql`, {
        credentials: "include",
      });
    }
    return _graphqlClient.request<T>(document, variables);
  },
};

export const publicGraphqlClient = {
  request: <T>(document: string, variables?: Record<string, unknown>): Promise<T> => {
    if (!_publicGraphqlClient) {
      _publicGraphqlClient = new GraphQLClient(`${getBaseUrl()}/graphql`, {
        credentials: "include",
      });
    }
    return _publicGraphqlClient.request<T>(document, variables);
  },
};
