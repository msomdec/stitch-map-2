import { useEffect, useState, useCallback, useRef } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useSessionStore } from '../stores/sessionStore';
import { ErrorNotification } from '../components/ErrorNotification';
import { ConfirmModal } from '../components/ConfirmModal';

export function WorkSessionPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const sessionId = Number(id);
  const {
    sessionView,
    sessionLoading,
    error,
    fetchSession,
    nextStitch,
    prevStitch,
    pauseSession,
    resumeSession,
    abandonSession,
    clearSession,
    clearError,
  } = useSessionStore();

  const [showAbandon, setShowAbandon] = useState(false);
  const touchStartX = useRef<number | null>(null);
  const actionInFlight = useRef(false);

  useEffect(() => {
    fetchSession(sessionId);
    return () => clearSession();
  }, [sessionId, fetchSession, clearSession]);

  const handleNext = useCallback(async () => {
    if (actionInFlight.current) return;
    if (!sessionView || sessionView.session.status !== 'active') return;
    actionInFlight.current = true;
    try {
      await nextStitch(sessionId);
    } finally {
      actionInFlight.current = false;
    }
  }, [sessionView, sessionId, nextStitch]);

  const handlePrev = useCallback(async () => {
    if (actionInFlight.current) return;
    if (!sessionView || sessionView.session.status !== 'active') return;
    actionInFlight.current = true;
    try {
      await prevStitch(sessionId);
    } finally {
      actionInFlight.current = false;
    }
  }, [sessionView, sessionId, prevStitch]);

  const handlePauseResume = useCallback(async () => {
    if (!sessionView) return;
    if (sessionView.session.status === 'active') {
      await pauseSession(sessionId);
    } else if (sessionView.session.status === 'paused') {
      await resumeSession(sessionId);
    }
  }, [sessionView, sessionId, pauseSession, resumeSession]);

  const handleAbandon = async () => {
    try {
      await abandonSession(sessionId);
      navigate('/dashboard');
    } catch {
      // Error in store
    }
  };

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
      switch (e.key) {
        case ' ':
        case 'ArrowRight':
          e.preventDefault();
          handleNext();
          break;
        case 'Backspace':
        case 'ArrowLeft':
          e.preventDefault();
          handlePrev();
          break;
        case 'p':
        case 'P':
          e.preventDefault();
          handlePauseResume();
          break;
        case 'Escape':
          e.preventDefault();
          setShowAbandon(true);
          break;
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleNext, handlePrev, handlePauseResume]);

  // Touch swipe
  const handleTouchStart = (e: React.TouchEvent) => {
    touchStartX.current = e.touches[0].clientX;
  };
  const handleTouchEnd = (e: React.TouchEvent) => {
    if (touchStartX.current === null) return;
    const diff = e.changedTouches[0].clientX - touchStartX.current;
    if (Math.abs(diff) > 50) {
      if (diff < 0) handleNext(); // swipe left = next
      else handlePrev(); // swipe right = prev
    }
    touchStartX.current = null;
  };

  if (sessionLoading && !sessionView) {
    return (
      <section className="section">
        <div className="container has-text-centered">
          <span className="loader"></span>
        </div>
      </section>
    );
  }

  if (!sessionView) {
    return (
      <section className="section">
        <div className="container">
          <ErrorNotification message={error ?? 'Session not found.'} />
          <Link to="/dashboard">Back to Dashboard</Link>
        </div>
      </section>
    );
  }

  const { session, pattern, progress } = sessionView;

  // Completed state
  if (session.status === 'completed') {
    return (
      <section className="section">
        <div className="container">
          <div className="columns is-centered">
            <div className="column is-8 has-text-centered">
              <div className="notification is-success is-light">
                <h2 className="title is-3">Pattern Complete!</h2>
                <p className="subtitle">
                  You've completed <strong>{pattern.name}</strong>. Well done!
                </p>
                <Link className="button is-primary mt-3" to="/dashboard">
                  Back to Dashboard
                </Link>
              </div>
            </div>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="section" onTouchStart={handleTouchStart} onTouchEnd={handleTouchEnd}>
      <div className="container">
        <div className="columns is-centered">
          <div className="column is-8">
            <ErrorNotification message={error} onDismiss={clearError} />

            {/* Paused banner */}
            {session.status === 'paused' && (
              <div className="notification is-warning">
                <strong>Session Paused</strong> — Press Resume or <kbd>P</kbd> to continue.
              </div>
            )}

            {/* Header */}
            <div className="level">
              <div className="level-left">
                <div>
                  <h1 className="title is-4">{pattern.name}</h1>
                  <p className="subtitle is-6">
                    {progress.groupLabel}
                    {progress.groupRepeatInfo && (
                      <span className="ml-2 has-text-grey">({progress.groupRepeatInfo})</span>
                    )}
                  </p>
                </div>
              </div>
              <div className="level-right">
                <div className="buttons">
                  <button
                    className={`button is-small ${session.status === 'active' ? 'is-warning' : 'is-success'}`}
                    onClick={handlePauseResume}
                  >
                    {session.status === 'active' ? 'Pause' : 'Resume'}
                  </button>
                  <button className="button is-small is-danger is-outlined" onClick={() => setShowAbandon(true)}>
                    Abandon
                  </button>
                </div>
              </div>
            </div>

            {/* Parts Overview Strip */}
            {progress.groups.length > 1 && (
              <div className="mb-4">
                <div className="tags are-medium" style={{ flexWrap: 'wrap', gap: '0.25rem' }}>
                  {progress.groups.map((g, i) => {
                    let cls = 'tag ';
                    if (g.status === 'completed') cls += 'is-success is-light';
                    else if (g.status === 'current') cls += 'is-info';
                    else cls += 'is-light';
                    let label = g.label;
                    if (g.status === 'completed') label = `\u2713 ${label}`;
                    if (g.status === 'current' && g.repeatCount > 1) {
                      label += ` (${g.currentRepeat}/${g.repeatCount})`;
                    }
                    return <span key={i} className={cls}>{label}</span>;
                  })}
                </div>
              </div>
            )}

            {/* Main Stitch Display */}
            <div className="box has-text-centered py-6" aria-live="assertive">
              <div className="columns is-vcentered is-mobile">
                <div className="column is-3 has-text-right">
                  {progress.prevAbbr && (
                    <span className="tag is-medium is-light">{progress.prevAbbr}</span>
                  )}
                </div>
                <div className="column is-6">
                  <h2 className="title is-1 mb-1">{progress.currentAbbr || '--'}</h2>
                  <p className="subtitle is-5 has-text-grey">{progress.currentName || ''}</p>
                </div>
                <div className="column is-3 has-text-left">
                  {progress.nextAbbr && (
                    <span className="tag is-medium is-light">{progress.nextAbbr}</span>
                  )}
                </div>
              </div>
            </div>

            {/* Group Progress */}
            {progress.groups.length > 1 && (
              <div className="mb-3">
                {progress.groups
                  .filter((g) => g.status === 'current')
                  .map((g, i) => (
                    <div key={i} className="mb-2">
                      <div className="is-flex is-justify-content-space-between is-size-7">
                        <span>{g.label}</span>
                        <span>{g.completedInGroup} / {g.totalInGroup}</span>
                      </div>
                      <progress
                        className="progress is-small is-info"
                        value={g.completedInGroup}
                        max={g.totalInGroup}
                      />
                    </div>
                  ))}
              </div>
            )}

            {/* Overall Progress */}
            <div className="mb-4">
              <div className="is-flex is-justify-content-space-between">
                <span>Overall Progress</span>
                <span>
                  {progress.completedStitches} / {progress.totalStitches} ({progress.percentage.toFixed(1)}%)
                </span>
              </div>
              <progress
                className="progress is-primary"
                value={progress.completedStitches}
                max={progress.totalStitches}
              />
            </div>

            {/* Navigation Buttons */}
            {session.status === 'active' && (
              <div className="buttons is-centered">
                <button className="button is-medium" onClick={handlePrev}>
                  &larr; Back
                </button>
                <button className="button is-medium is-primary" onClick={handleNext}>
                  Next &rarr;
                </button>
              </div>
            )}

            {/* Keyboard Help */}
            <div className="has-text-centered is-size-7 has-text-grey mt-4">
              <kbd>Space</kbd> / <kbd>&rarr;</kbd> Next &nbsp;|&nbsp;
              <kbd>Backspace</kbd> / <kbd>&larr;</kbd> Previous &nbsp;|&nbsp;
              <kbd>P</kbd> Pause/Resume &nbsp;|&nbsp;
              <kbd>Esc</kbd> Abandon
            </div>

            {/* Abandon Modal */}
            <ConfirmModal
              isOpen={showAbandon}
              title="Abandon Session"
              message="Are you sure you want to abandon this session? Your progress will be lost."
              confirmLabel="Abandon"
              onConfirm={handleAbandon}
              onCancel={() => setShowAbandon(false)}
            />
          </div>
        </div>
      </div>
    </section>
  );
}
