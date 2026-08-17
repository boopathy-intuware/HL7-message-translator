import { NavLink, Outlet } from 'react-router-dom'

function navLinkClassName({ isActive }: { isActive: boolean }): string {
  return `app-nav__link${isActive ? ' app-nav__link--active' : ''}`
}

export function Layout() {
  return (
    <div className="app-layout">
      <nav className="app-nav">
        <div className="app-nav__brand">HL7 → FHIR</div>
        <NavLink to="/" end className={navLinkClassName}>
          Convert
        </NavLink>
        <NavLink to="/inbox" className={navLinkClassName}>
          Inbox
        </NavLink>
        <NavLink to="/fhir" className={navLinkClassName}>
          FHIR Search
        </NavLink>
      </nav>
      <main className="app-main">
        <Outlet />
      </main>
    </div>
  )
}