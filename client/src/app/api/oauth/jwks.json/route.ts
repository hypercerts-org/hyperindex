import { NextResponse } from "next/server";
import { getJwks } from "@/lib/auth/client";

export const dynamic = "force-dynamic";

export async function GET() {
  try {
    const jwks = await getJwks();
    
    if (!jwks) {
      // Return empty keyset if no private key configured (public client mode)
      return NextResponse.json(
        { keys: [] },
        {
          headers: {
            "Content-Type": "application/json",
            "Cache-Control": "public, max-age=3600",
          },
        }
      );
    }

    return NextResponse.json(jwks, {
      headers: {
        "Content-Type": "application/json",
        "Cache-Control": "public, max-age=3600",
      },
    });
  } catch (error) {
    console.error("Failed to get JWKS:", error);
    return NextResponse.json(
      { error: "Failed to get JWKS" },
      { status: 500 }
    );
  }
}
