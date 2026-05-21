import { NextResponse } from "next/server";
import { loadSiteConfig } from "../../../lib/site-config";

export async function GET() {
  return NextResponse.json(await loadSiteConfig());
}
