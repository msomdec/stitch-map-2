import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { usePatternStore } from '../stores/patternStore';
import { useSessionStore } from '../stores/sessionStore';
import { ConfirmModal } from '../components/ConfirmModal';
import type { Pattern } from '../types';

function computeStitchCount(pattern: Pattern): number {
  let total = 0;
  for (const group of pattern.instructionGroups || []) {
    let groupCount = 0;
    for (const entry of group.stitchEntries || []) {
      groupCount += entry.count * entry.repeatCount;
    }
    total += groupCount * group.repeatCount;
  }
  return total;
}

export function PatternListPage() {
  const { patterns, loading, error, fetchPatterns, deletePattern, duplicatePattern, clearError } = usePatternStore();
  const { startSession } = useSessionStore();
  const navigate = useNavigate();
  const [deleteTarget, setDeleteTarget] = useState<Pattern | null>(null);

  useEffect(() => {
    fetchPatterns();
  }, [fetchPatterns]);

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deletePattern(deleteTarget.id);
      setDeleteTarget(null);
      fetchPatterns();
    } catch {
      // Error shown in store
    }
  };

  const handleDuplicate = async (id: number) => {
    try {
      await duplicatePattern(id);
      fetchPatterns();
    } catch {
      // Error shown in store
    }
  };

  const handleStartSession = async (patternId: number) => {
    try {
      const session = await startSession(patternId);
      navigate(`/sessions/${session.id}`);
    } catch {
      // Error shown in store
    }
  };

  return (
    <section className="section">
      <div className="container">
        <div className="level">
          <div className="level-left">
            <h1 className="title">My Patterns</h1>
          </div>
          <div className="level-right">
            <Link className="button is-primary" to="/patterns/new">
              New Pattern
            </Link>
          </div>
        </div>

        {error && (
          <div className="notification is-danger">
            <button className="delete" onClick={clearError}></button>
            {error}
          </div>
        )}

        {loading && patterns.length === 0 && <p>Loading patterns...</p>}

        {!loading && patterns.length === 0 && (
          <div className="notification is-light">
            You haven't created any patterns yet.{' '}
            <Link to="/patterns/new">Create your first pattern</Link>.
          </div>
        )}

        <div className="columns is-multiline">
          {patterns.map((pattern) => {
            const stitchCount = computeStitchCount(pattern);
            const groupCount = (pattern.instructionGroups || []).length;
            return (
              <div key={pattern.id} className="column is-4">
                <div className="card">
                  <div className="card-content">
                    <p className="title is-5">{pattern.name}</p>
                    <p className="subtitle is-6 has-text-grey">
                      {pattern.patternType === 'round' ? 'Rounds' : 'Rows'}
                      {pattern.hookSize && ` · ${pattern.hookSize}`}
                      {pattern.yarnWeight && ` · ${pattern.yarnWeight}`}
                    </p>
                    {pattern.description && (
                      <p className="mb-3">
                        {pattern.description.length > 100
                          ? pattern.description.slice(0, 100) + '...'
                          : pattern.description}
                      </p>
                    )}
                    <div className="tags">
                      <span className="tag is-info is-light">{groupCount} groups</span>
                      <span className="tag is-primary is-light">{stitchCount} stitches</span>
                      {pattern.difficulty && (
                        <span className="tag is-warning is-light">{pattern.difficulty}</span>
                      )}
                    </div>
                  </div>
                  <footer className="card-footer">
                    <Link to={`/patterns/${pattern.id}`} className="card-footer-item">
                      View
                    </Link>
                    <Link to={`/patterns/${pattern.id}/edit`} className="card-footer-item">
                      Edit
                    </Link>
                    <a className="card-footer-item" onClick={() => handleStartSession(pattern.id)}>
                      Start
                    </a>
                  </footer>
                  <footer className="card-footer">
                    <a className="card-footer-item" onClick={() => handleDuplicate(pattern.id)}>
                      Duplicate
                    </a>
                    <a
                      className="card-footer-item has-text-danger"
                      onClick={() => setDeleteTarget(pattern)}
                    >
                      Delete
                    </a>
                  </footer>
                </div>
              </div>
            );
          })}
        </div>

        <ConfirmModal
          isOpen={!!deleteTarget}
          title="Delete Pattern"
          message={`Are you sure you want to delete "${deleteTarget?.name}"? This cannot be undone.`}
          onConfirm={handleDelete}
          onCancel={() => setDeleteTarget(null)}
        />
      </div>
    </section>
  );
}
