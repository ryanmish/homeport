// Homeport Logo - A stylized anchor/port symbol
export function Logo({ size = 32, className = '' }: { size?: number; className?: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
    >
      {/* Outer ring - represents the port/harbor */}
      <circle
        cx="16"
        cy="16"
        r="14"
        stroke="currentColor"
        strokeWidth="2"
        fill="none"
        opacity="0.2"
      />
      {/* Inner filled circle - the home base */}
      <circle
        cx="16"
        cy="16"
        r="6"
        fill="currentColor"
      />
      {/* Connection lines - representing ports/connections */}
      <path
        d="M16 2 L16 10"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
      />
      <path
        d="M16 22 L16 30"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
      />
      <path
        d="M2 16 L10 16"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
      />
      <path
        d="M22 16 L30 16"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
      />
      {/* Small dots at the ends - representing active connections */}
      <circle cx="16" cy="4" r="2" fill="currentColor" opacity="0.6" />
      <circle cx="16" cy="28" r="2" fill="currentColor" opacity="0.6" />
      <circle cx="4" cy="16" r="2" fill="currentColor" opacity="0.6" />
      <circle cx="28" cy="16" r="2" fill="currentColor" opacity="0.6" />
    </svg>
  )
}

// Alternative minimal logo for small spaces
export function LogoMini({ size = 24, className = '' }: { size?: number; className?: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
    >
      <circle cx="12" cy="12" r="4" fill="currentColor" />
      <path d="M12 2v6M12 16v6M2 12h6M16 12h6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  )
}
