import { NextRequest, NextResponse } from "next/server";

const BLUESKY_PUBLIC_API = "https://public.api.bsky.app";

type BlueskyActor = {
  did: string;
  handle: string;
  displayName?: string;
  avatar?: string;
};

type BlueskyTypeaheadResponse = {
  actors?: BlueskyActor[];
};

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const query = searchParams.get("q")?.trim() ?? "";
    const limitParam = searchParams.get("limit");
    const parsedLimit = Number.parseInt(limitParam ?? "10", 10);
    const limit = Number.isNaN(parsedLimit)
      ? 10
      : Math.max(1, Math.min(100, parsedLimit));

    if (!query) {
      return NextResponse.json({ actors: [] });
    }

    const url = new URL(
      "/xrpc/app.bsky.actor.searchActorsTypeahead",
      BLUESKY_PUBLIC_API,
    );
    url.searchParams.set("q", query);
    url.searchParams.set("limit", String(limit));

    const response = await fetch(url.toString(), {
      method: "GET",
      headers: {
        Accept: "application/json",
      },
      cache: "no-store",
    });

    if (!response.ok) {
      return NextResponse.json(
        { error: `Bluesky API returned ${response.status}`, actors: [] },
        { status: response.status },
      );
    }

    const data = (await response.json()) as BlueskyTypeaheadResponse;
    const actors = (data.actors ?? []).map((actor) => ({
      did: actor.did,
      handle: actor.handle,
      displayName: actor.displayName,
      avatar: actor.avatar,
    }));

    return NextResponse.json({ actors });
  } catch (error) {
    console.error("Bluesky typeahead proxy error:", error);
    return NextResponse.json(
      { error: "Failed to fetch actor suggestions", actors: [] },
      { status: 500 },
    );
  }
}
