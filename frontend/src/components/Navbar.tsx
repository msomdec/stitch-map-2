import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';

export function Navbar() {
  const { user, logout } = useAuthStore();
  const navigate = useNavigate();
  const [menuOpen, setMenuOpen] = useState(false);

  const handleLogout = async () => {
    await logout();
    navigate('/');
  };

  return (
    <nav className="navbar is-dark" role="navigation" aria-label="main navigation">
      <div className="navbar-brand">
        <Link className="navbar-item has-text-weight-bold" to="/">
          StitchMap
        </Link>
        <button
          className={`navbar-burger ${menuOpen ? 'is-active' : ''}`}
          aria-label="menu"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen(!menuOpen)}
        >
          <span aria-hidden="true"></span>
          <span aria-hidden="true"></span>
          <span aria-hidden="true"></span>
          <span aria-hidden="true"></span>
        </button>
      </div>

      <div className={`navbar-menu ${menuOpen ? 'is-active' : ''}`}>
        {user && (
          <div className="navbar-start">
            <Link className="navbar-item" to="/patterns" onClick={() => setMenuOpen(false)}>
              Patterns
            </Link>
            <Link className="navbar-item" to="/stitches" onClick={() => setMenuOpen(false)}>
              Stitch Library
            </Link>
            <Link className="navbar-item" to="/dashboard" onClick={() => setMenuOpen(false)}>
              Dashboard
            </Link>
          </div>
        )}

        <div className="navbar-end">
          {user ? (
            <div className="navbar-item has-dropdown is-hoverable">
              <a className="navbar-link">{user.displayName}</a>
              <div className="navbar-dropdown is-right">
                <a className="navbar-item" onClick={handleLogout}>
                  Logout
                </a>
              </div>
            </div>
          ) : (
            <>
              <div className="navbar-item">
                <div className="buttons">
                  <Link className="button is-light" to="/login" onClick={() => setMenuOpen(false)}>
                    Log in
                  </Link>
                  <Link className="button is-primary" to="/register" onClick={() => setMenuOpen(false)}>
                    Register
                  </Link>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </nav>
  );
}
