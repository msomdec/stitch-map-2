import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';

export function RegisterPage() {
  const { register, loading, error, clearError, user } = useAuthStore();
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  if (user) {
    navigate('/', { replace: true });
    return null;
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await register({ email, displayName, password, confirmPassword });
      navigate('/login');
    } catch {
      // Error is set in store
    }
  };

  return (
    <section className="section">
      <div className="container">
        <div className="columns is-centered">
          <div className="column is-5">
            <h1 className="title">Register</h1>

            {error && (
              <div className="notification is-danger">
                <button className="delete" onClick={clearError}></button>
                {error}
              </div>
            )}

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
                <label className="label">Display Name</label>
                <div className="control">
                  <input
                    className="input"
                    type="text"
                    value={displayName}
                    onChange={(e) => setDisplayName(e.target.value)}
                    required
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
                    minLength={8}
                  />
                </div>
                <p className="help">Must be at least 8 characters</p>
              </div>

              <div className="field">
                <label className="label">Confirm Password</label>
                <div className="control">
                  <input
                    className="input"
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
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
                    Register
                  </button>
                </div>
              </div>
            </form>

            <p className="has-text-centered mt-4">
              Already have an account?{' '}
              <Link to="/login">Log in</Link>
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}
