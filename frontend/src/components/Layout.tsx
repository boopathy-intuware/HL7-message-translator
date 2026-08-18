import { NavLink, Outlet } from 'react-router-dom'
import { InboxIcon, LogoIcon, SearchIcon, SwapIcon } from './icons'

function navLinkClassName({ isActive }: { isActive: boolean }): string {
  return `app-nav__link${isActive ? ' app-nav__link--active' : ''}`
}

export function Layout() {
  return (
    <div className="app-layout">
      <nav className="app-nav">
        <div className="app-nav__brand">
          <span className="app-nav__brand-icon">
            <LogoIcon size={16} />
          </span>
          HL7 → FHIR
        </div>
        <NavLink to="/" end className={navLinkClassName}>
          <SwapIcon size={17} />
          Convert
        </NavLink>
        <NavLink to="/inbox" className={navLinkClassName}>
          <InboxIcon size={17} />
          Inbox
        </NavLink>
        <NavLink to="/fhir" className={navLinkClassName}>
          <SearchIcon size={17} />
          FHIR Search
        </NavLink>
      </nav>
      <main className="app-main">
        <Outlet />
      </main>
    </div>
  )
}