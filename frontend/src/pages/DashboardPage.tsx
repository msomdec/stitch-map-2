import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';
import { useSessionStore } from '../stores/sessionStore';
import { ErrorNotification } from '../components/ErrorNotification';

export function DashboardPage() {
  const user = useAuthStore((s) => s.user);
  const { dashboard, dashboardLoading, error, clearError, fetchDashboard, loadMoreCompleted } = useSessionStore();

  useEffect(() => {
    fetchDashboard();
  }, [fetchDashboard]);

  if (dashboardLoading && !dashboard) {
    return (
      <section className="section">
        <div className="container has-text-centered">
          <span className="loader"></span>
        </div>
      </section>
    );
  }

  const hasMore = dashboard && dashboard.completedSessions.length < dashboard.totalCompleted;

  return (
    <section className="section">
      <div className="container">
        <h1 className="title">Welcome, {user?.displayName}</h1>

        <ErrorNotification message={error} onDismiss={clearError} />

        {/* Active Sessions */}
        <h2 className="title is-4 mt-5">Active Sessions</h2>
        {(!dashboard?.activeSessions || dashboard.activeSessions.length === 0) ? (
          <div className="notification is-light">
            No active sessions.{' '}
            <Link to="/patterns">Start one from your patterns</Link>.
          </div>
        ) : (
          <div className="columns is-multiline">
            {dashboard.activeSessions.map((session) => (
              <div key={session.id} className="column is-4">
                <div className="card">
                  <div className="card-content">
                    <p className="title is-5">
                      {dashboard.patternNames[String(session.patternId)] || 'Unknown Pattern'}
                    </p>
                    <p>
                      <span className={`tag ${session.status === 'active' ? 'is-success' : 'is-warning'}`}>
                        {session.status === 'active' ? 'Active' : 'Paused'}
                      </span>
                    </p>
                  </div>
                  <div className="card-footer">
                    <Link to={`/sessions/${session.id}`} className="card-footer-item">
                      Resume
                    </Link>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Completed Sessions */}
        <h2 className="title is-4 mt-5">Completed Sessions</h2>
        {(!dashboard?.completedSessions || dashboard.completedSessions.length === 0) ? (
          <div className="notification is-light">No completed sessions yet.</div>
        ) : (
          <>
            <div className="columns is-multiline">
              {dashboard.completedSessions.map((session) => (
                <div key={session.id} className="column is-4">
                  <div className="card">
                    <div className="card-content">
                      <p className="title is-5">
                        {dashboard.patternNames[String(session.patternId)] || 'Unknown Pattern'}
                      </p>
                      <p>
                        <span className="tag is-info">Completed</span>
                        {session.completedAt && (
                          <span className="ml-2 is-size-7 has-text-grey">
                            {new Date(session.completedAt).toLocaleDateString()}
                          </span>
                        )}
                      </p>
                    </div>
                    <div className="card-footer">
                      <Link to={`/patterns/${session.patternId}`} className="card-footer-item">
                        View Pattern
                      </Link>
                    </div>
                  </div>
                </div>
              ))}
            </div>
            {hasMore && (
              <div className="has-text-centered mt-4">
                <button
                  className="button is-light"
                  onClick={() => loadMoreCompleted(dashboard!.completedSessions.length)}
                >
                  Load More
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </section>
  );
}
