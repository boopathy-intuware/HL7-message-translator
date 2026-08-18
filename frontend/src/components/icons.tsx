import type { ReactNode, SVGProps } from 'react'

type IconProps = SVGProps<SVGSVGElement> & { size?: number }

function Icon({ size = 18, children, ...props }: IconProps & { children: ReactNode }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.8}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      {...props}
    >
      {children}
    </svg>
  )
}

export function LogoIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M3 12h4l2-7 4 14 2-7h6" />
    </Icon>
  )
}

export function SwapIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M7 4v13" />
      <path d="M3 13l4 4 4-4" />
      <path d="M17 20V7" />
      <path d="M21 11l-4-4-4 4" />
    </Icon>
  )
}

export function InboxIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M3.5 11h4.2l1.5 2.5h5.6l1.5-2.5h4.2" />
      <path d="M5 4.5h14L21 11v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-6z" />
    </Icon>
  )
}

export function SearchIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="M20 20l-4.8-4.8" />
    </Icon>
  )
}

export function SendIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M21 3 3 10.5l7.5 2.5L13 20.5z" />
      <path d="M21 3 10.5 13.5" />
    </Icon>
  )
}

export function ArrowLeftIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M19 12H5" />
      <path d="M11 6l-6 6 6 6" />
    </Icon>
  )
}

export function CheckCircleIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M8.5 12.2l2.4 2.4 4.6-5.2" />
    </Icon>
  )
}

export function AlertCircleIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.5v5.2" />
      <path d="M12 16.3h.01" />
    </Icon>
  )
}

export function FileTextIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M7 3.5h7l3.5 3.5V20a.5.5 0 0 1-.5.5H7a.5.5 0 0 1-.5-.5V4a.5.5 0 0 1 .5-.5z" />
      <path d="M14 3.5V7h3.5" />
      <path d="M9 12h6" />
      <path d="M9 15.5h6" />
    </Icon>
  )
}

export function BraceIcon(props: IconProps) {
  return (
    <Icon {...props}>
      <path d="M9 3.5c-2 0-2.5 1-2.5 2.7v3c0 1.3-.5 2-1.9 2.3v1c1.4.3 1.9 1 1.9 2.3v3c0 1.7.5 2.7 2.5 2.7" />
      <path d="M15 3.5c2 0 2.5 1 2.5 2.7v3c0 1.3.5 2 1.9 2.3v1c-1.4.3-1.9 1-1.9 2.3v3c0 1.7-.5 2.7-2.5 2.7" />
    </Icon>
  )
}
