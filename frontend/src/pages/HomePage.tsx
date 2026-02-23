import { Link } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';

export function HomePage() {
  const user = useAuthStore((s) => s.user);

  if (user) {
    return (
      <section className="section">
        <div className="container">
          <h1 className="title">Welcome back, {user.displayName}!</h1>
          <div className="columns">
            <div className="column is-4">
              <div className="box">
                <h2 className="title is-5">My Patterns</h2>
                <p>Create and manage your crochet patterns.</p>
                <Link className="button is-primary mt-3" to="/patterns">
                  View Patterns
                </Link>
              </div>
            </div>
            <div className="column is-4">
              <div className="box">
                <h2 className="title is-5">Dashboard</h2>
                <p>Track your active work sessions.</p>
                <Link className="button is-info mt-3" to="/dashboard">
                  Go to Dashboard
                </Link>
              </div>
            </div>
            <div className="column is-4">
              <div className="box">
                <h2 className="title is-5">Stitch Library</h2>
                <p>Browse stitches and add custom ones.</p>
                <Link className="button is-link mt-3" to="/stitches">
                  View Library
                </Link>
              </div>
            </div>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="hero is-medium is-primary">
      <div className="hero-body">
        <div className="container has-text-centered">
          <h1 className="title is-1">StitchMap</h1>
          <h2 className="subtitle is-4">
            Build, manage, and track your crochet patterns stitch-by-stitch
          </h2>
          <div className="buttons is-centered mt-5">
            <Link className="button is-light is-medium" to="/register">
              Get Started
            </Link>
            <Link className="button is-outlined is-light is-medium" to="/login">
              Log In
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}
