import { useState, type FormEvent } from 'react';
import { Link, Navigate, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';
import { ErrorNotification } from '../components/ErrorNotification';

export function LoginPage() {
  const { login, loading, error, clearError, user } = useAuthStore();
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  // Use <Navigate> instead of navigate() to avoid side effects during render.
  if (user) {
    return <Navigate to="/" replace />;
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await login({ email, password });
      navigate('/');
    } catch {
      // Error is set in store
    }
  };

  return (
    <section className="section">
      <div className="container">
        <div className="columns is-centered">
          <div className="column is-5">
            <h1 className="title">Log In</h1>

            <ErrorNotification message={error} onDismiss={clearError} />

            <form onSubmit={handleSubmit}>
              <div className="field">
                <label className="label">Email</label>
                <div className="control">
                  <input
                    className="input"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    required
                    autoFocus
                  />
                </div>
              </div>

              <div className="field">
                <label className="label">Password</label>
                <div className="control">
                  <input
                    className="input"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                  />
                </div>
              </div>

              <div className="field">
                <div className="control">
                  <button
                    className={`button is-primary is-fullwidth ${loading ? 'is-loading' : ''}`}
                    type="submit"
                    disabled={loading}
                  >
                    Log In
                  </button>
                </div>
              </div>
            </form>

            <p className="has-text-centered mt-4">
              Don't have an account?{' '}
              <Link to="/register">Register</Link>
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}
