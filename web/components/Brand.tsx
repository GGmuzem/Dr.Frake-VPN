import Image from "next/image";

export function Brand() {
  return (
    <a className="brand" href="/">
      <Image src="/brand-icon.png" width={48} height={48} alt="FBLink VPN" priority />
      <span>
        FBLink <span>VPN</span>
      </span>
    </a>
  );
}
