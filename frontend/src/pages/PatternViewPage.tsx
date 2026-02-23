import { useEffect } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { usePatternStore } from '../stores/patternStore';
import { useStitchStore } from '../stores/stitchStore';
import { useSessionStore } from '../stores/sessionStore';
import { imagesApi } from '../api/images';

function categoryTagClass(category: string): string {
  switch (category) {
    case 'basic': return 'is-info';
    case 'increase': return 'is-success';
    case 'decrease': return 'is-warning';
    case 'post': return 'is-primary';
    case 'advanced': return 'is-link';
    case 'specialty': return 'is-danger';
    case 'action': return 'is-dark';
    default: return 'is-light';
  }
}

export function PatternViewPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const {
    currentPattern: pattern,
    currentPatternText,
    currentStitchCount,
    currentGroupImages,
    loading,
    error,
    fetchPattern,
    duplicatePattern,
    clearCurrent,
  } = usePatternStore();
  const { allStitches, fetchAllStitches } = useStitchStore();
  const { startSession } = useSessionStore();

  useEffect(() => {
    if (id) {
      fetchPattern(Number(id));
      fetchAllStitches();
    }
    return () => clearCurrent();
  }, [id, fetchPattern, fetchAllStitches, clearCurrent]);

  const stitchLookup = Object.fromEntries(allStitches.map((s) => [s.id, s]));

  const handleDuplicate = async () => {
    if (!pattern) return;
    try {
      await duplicatePattern(pattern.id);
      navigate('/patterns');
    } catch {
      // Error in store
    }
  };

  const handleStartSession = async () => {
    if (!pattern) return;
    try {
      const session = await startSession(pattern.id);
      navigate(`/sessions/${session.id}`);
    } catch {
      // Error in store
    }
  };

  if (loading) {
    return (
      <section className="section">
        <div className="container has-text-centered">
          <p>Loading pattern...</p>
        </div>
      </section>
    );
  }

  if (error || !pattern) {
    return (
      <section className="section">
        <div className="container">
          <div className="notification is-danger">{error || 'Pattern not found'}</div>
          <Link to="/patterns">Back to Patterns</Link>
        </div>
      </section>
    );
  }

  return (
    <section className="section">
      <div className="container">
        <div className="level">
          <div className="level-left">
            <div>
              <h1 className="title">{pattern.name}</h1>
              <div className="tags">
                <span className="tag is-medium is-info">
                  {pattern.patternType === 'round' ? 'Rounds' : 'Rows'}
                </span>
                {pattern.difficulty && (
                  <span className="tag is-medium is-warning">{pattern.difficulty}</span>
                )}
                {pattern.hookSize && (
                  <span className="tag is-medium is-light">Hook: {pattern.hookSize}</span>
                )}
                {pattern.yarnWeight && (
                  <span className="tag is-medium is-light">Yarn: {pattern.yarnWeight}</span>
                )}
                <span className="tag is-medium is-primary">{currentStitchCount} stitches</span>
              </div>
            </div>
          </div>
          <div className="level-right">
            <div className="buttons">
              <Link className="button" to="/patterns">Back to Patterns</Link>
              <Link className="button is-info" to={`/patterns/${pattern.id}/edit`}>Edit</Link>
              <button className="button is-light" onClick={handleDuplicate}>Duplicate</button>
              <button className="button is-primary" onClick={handleStartSession}>Start Session</button>
            </div>
          </div>
        </div>

        {pattern.description && (
          <div className="content mb-5">
            <p>{pattern.description}</p>
          </div>
        )}

        {/* Pattern Text */}
        {currentPatternText && (
          <div className="box mb-5">
            <h2 className="title is-5">Pattern Text</h2>
            <pre style={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace' }}>
              {currentPatternText}
            </pre>
          </div>
        )}

        {/* Instruction Groups */}
        <h2 className="title is-5">Instruction Groups</h2>
        {(pattern.instructionGroups || []).map((group, gi) => {
          const images = currentGroupImages?.[String(group.id)] || [];
          return (
            <div key={group.id || gi} className="card mb-4">
              <div className="card-content">
                <div className="level">
                  <div className="level-left">
                    <p className="title is-6">{group.label}</p>
                  </div>
                  <div className="level-right">
                    <div className="tags">
                      {group.repeatCount > 1 && (
                        <span className="tag is-info">Repeat x{group.repeatCount}</span>
                      )}
                      {group.expectedCount != null && (
                        <span className="tag is-light">Expected: {group.expectedCount}</span>
                      )}
                    </div>
                  </div>
                </div>

                {group.notes && <p className="is-italic has-text-grey mb-3">{group.notes}</p>}

                <div className="tags are-medium">
                  {(group.stitchEntries || []).map((entry, ei) => {
                    const stitch = stitchLookup[entry.stitchId];
                    const abbr = stitch?.abbreviation || '?';
                    const tagClass = stitch ? categoryTagClass(stitch.category) : 'is-light';
                    let label = entry.count > 1 ? `${entry.count} ${abbr}` : abbr;
                    if (entry.repeatCount > 1) label += ` x${entry.repeatCount}`;
                    if (entry.intoStitch) label += ` ${entry.intoStitch}`;
                    return (
                      <span key={entry.id || ei} className={`tag ${tagClass}`}>
                        {label}
                      </span>
                    );
                  })}
                </div>

                {/* Images */}
                {images.length > 0 && (
                  <div className="mt-3">
                    <div className="is-flex is-flex-wrap-wrap" style={{ gap: '0.5rem' }}>
                      {images.map((img) => (
                        <a key={img.id} href={imagesApi.url(img.id)} target="_blank" rel="noopener noreferrer">
                          <figure className="image is-128x128">
                            <img src={imagesApi.url(img.id)} alt={img.filename} style={{ objectFit: 'cover', borderRadius: '4px' }} />
                          </figure>
                        </a>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}
