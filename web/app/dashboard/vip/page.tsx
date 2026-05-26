import { Suspense } from "react";
import { VipPageClient } from "../../../components/VipPageClient";

export default function VipPage() {
  return (
    <Suspense>
      <VipPageClient />
    </Suspense>
  );
}
