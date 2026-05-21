import Image from "next/image";

type BrandProps = {
  href?: string;
};

export function Brand({ href = "/" }: BrandProps) {
  return (
    <a className="brand" href={href} aria-label="FBLink VPN">
      <span className="brand-mark" aria-hidden="true">
        <Image src="/brand-icon.png" width={40} height={40} alt="" priority />
      </span>
      <span className="brand-text">
        FBLink <span>VPN</span>
      </span>
    </a>
  );
}
