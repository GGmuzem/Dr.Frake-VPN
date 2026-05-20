"use client";

import { DitheringShader } from "@/components/ui/dithering-shader";
import { cn } from "@/lib/utils";

export const Component = ({ className }: { className?: string }) => {
  return (
    <div className={cn("relative h-full w-full overflow-hidden", className)}>
      <DitheringShader
        className="h-full w-full"
        colorBack="#070707"
        colorFront="#EAB308"
        height={720}
        pxSize={4}
        shape="wave"
        speed={0.34}
        type="8x8"
        width={1200}
      />
    </div>
  );
};
