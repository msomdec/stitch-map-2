import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { usePatternStore } from '../stores/patternStore';
import { useStitchStore } from '../stores/stitchStore';
import { ErrorNotification } from '../components/ErrorNotification';
import { ConfirmModal } from '../components/ConfirmModal';
import type { PatternCreateRequest } from '../api/patterns';

interface LocalStitchEntry {
  key: string;
  stitchId: number;
  count: number;
  repeatCount: number;
}

interface LocalInstructionGroup {
  key: string;
  label: string;
  repeatCount: number;
  expectedCount: string;
  notes: string;
  entries: LocalStitchEntry[];
}

let nextKey = 0;
function genKey() {
  return `k-${nextKey++}`;
}

function newEntry(): LocalStitchEntry {
  return { key: genKey(), stitchId: 0, count: 1, repeatCount: 1 };
}

function newGroup(): LocalInstructionGroup {
  return {
    key: genKey(),
    label: '',
    repeatCount: 1,
    expectedCount: '',
    notes: '',
    entries: [newEntry()],
  };
}

export function PatternEditorPage() {
  const { id } = useParams<{ id: string }>();
  const isEdit = !!id;
  const navigate = useNavigate();
  const { currentPattern, fetchPattern, createPattern, updatePattern, previewPattern, clearCurrent, error, clearError } = usePatternStore();
  const { allStitches, fetchAllStitches } = useStitchStore();

  // Form state
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [patternType, setPatternType] = useState<string>('round');
  const [hookSize, setHookSize] = useState('');
  const [yarnWeight, setYarnWeight] = useState('');
  const [difficulty, setDifficulty] = useState('');
  const [groups, setGroups] = useState<LocalInstructionGroup[]>([newGroup()]);

  // Modal state
  const [showSave, setShowSave] = useState(false);
  const [showCancel, setShowCancel] = useState(false);
  const [showPreview, setShowPreview] = useState(false);
  const [previewText, setPreviewText] = useState('');
  const [previewStitchCount, setPreviewStitchCount] = useState(0);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    fetchAllStitches();
    if (isEdit) {
      fetchPattern(Number(id));
    }
    return () => clearCurrent();
  }, [id, isEdit, fetchPattern, fetchAllStitches, clearCurrent]);

  // Populate form when pattern loads in edit mode
  useEffect(() => {
    if (isEdit && currentPattern) {
      setName(currentPattern.name);
      setDescription(currentPattern.description);
      setPatternType(currentPattern.patternType);
      setHookSize(currentPattern.hookSize);
      setYarnWeight(currentPattern.yarnWeight);
      setDifficulty(currentPattern.difficulty);
      setGroups(
        currentPattern.instructionGroups.map((g) => ({
          key: genKey(),
          label: g.label,
          repeatCount: g.repeatCount,
          expectedCount: g.expectedCount != null ? String(g.expectedCount) : '',
          notes: g.notes,
          entries: g.stitchEntries.map((e) => ({
            key: genKey(),
            stitchId: e.stitchId,
            count: e.count,
            repeatCount: e.repeatCount,
          })),
        }))
      );
    }
  }, [isEdit, currentPattern]);

  const buildPayload = (): PatternCreateRequest => ({
    name,
    description,
    patternType,
    hookSize,
    yarnWeight,
    difficulty,
    instructionGroups: groups.map((g) => ({
      label: g.label,
      repeatCount: g.repeatCount,
      expectedCount: g.expectedCount ? parseInt(g.expectedCount, 10) || null : null,
      notes: g.notes,
      stitchEntries: g.entries.map((e) => ({
        stitchId: e.stitchId,
        count: e.count,
        intoStitch: '',
        repeatCount: e.repeatCount,
      })),
    })),
  });

  const handleSave = async () => {
    setSaving(true);
    try {
      if (isEdit) {
        await updatePattern(Number(id), buildPayload());
      } else {
        await createPattern(buildPayload());
      }
      navigate('/patterns');
    } catch {
      // Error set in store
    } finally {
      setSaving(false);
      setShowSave(false);
    }
  };

  const handlePreview = async () => {
    try {
      const result = await previewPattern(buildPayload());
      setPreviewText(result.text);
      setPreviewStitchCount(result.stitchCount);
      setShowPreview(true);
    } catch {
      // Error in store
    }
  };

  // Group operations
  const addGroup = () => setGroups([...groups, newGroup()]);
  const removeGroup = (idx: number) => setGroups(groups.filter((_, i) => i !== idx));
  const updateGroup = (idx: number, field: string, value: string | number) => {
    setGroups(groups.map((g, i) => (i === idx ? { ...g, [field]: value } : g)));
  };

  // Entry operations
  const addEntry = (gi: number) => {
    setGroups(groups.map((g, i) =>
      i === gi ? { ...g, entries: [...g.entries, newEntry()] } : g
    ));
  };
  const removeEntry = (gi: number, ei: number) => {
    setGroups(groups.map((g, i) =>
      i === gi ? { ...g, entries: g.entries.filter((_, j) => j !== ei) } : g
    ));
  };
  const updateEntry = (gi: number, ei: number, field: string, value: string | number) => {
    setGroups(groups.map((g, i) =>
      i === gi
        ? {
            ...g,
            entries: g.entries.map((e, j) => (j === ei ? { ...e, [field]: value } : e)),
          }
        : g
    ));
  };

  if (isEdit && !currentPattern && !error) {
    return (
      <section className="section">
        <div className="container has-text-centered"><span className="loader"></span></div>
      </section>
    );
  }

  return (
    <section className="section">
      <div className="container">
        <h1 className="title">{isEdit ? 'Edit Pattern' : 'New Pattern'}</h1>

        <ErrorNotification message={error} onDismiss={clearError} />

        {/* Pattern Metadata */}
        <div className="box">
          <div className="columns">
            <div className="column is-6">
              <div className="field">
                <label className="label">Pattern Name *</label>
                <div className="control">
                  <input className="input" type="text" value={name} onChange={(e) => setName(e.target.value)} required />
                </div>
              </div>
            </div>
            <div className="column is-3">
              <div className="field">
                <label className="label">Type</label>
                <div className="control">
                  <div className="select is-fullwidth">
                    <select value={patternType} onChange={(e) => setPatternType(e.target.value)}>
                      <option value="round">Rounds</option>
                      <option value="row">Rows</option>
                    </select>
                  </div>
                </div>
              </div>
            </div>
            <div className="column is-3">
              <div className="field">
                <label className="label">Difficulty</label>
                <div className="control">
                  <div className="select is-fullwidth">
                    <select value={difficulty} onChange={(e) => setDifficulty(e.target.value)}>
                      <option value="">--</option>
                      <option value="Beginner">Beginner</option>
                      <option value="Intermediate">Intermediate</option>
                      <option value="Advanced">Advanced</option>
                      <option value="Expert">Expert</option>
                    </select>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div className="columns">
            <div className="column is-4">
              <div className="field">
                <label className="label">Hook Size</label>
                <div className="control">
                  <input className="input" type="text" placeholder="e.g., 5.0mm" value={hookSize} onChange={(e) => setHookSize(e.target.value)} />
                </div>
              </div>
            </div>
            <div className="column is-4">
              <div className="field">
                <label className="label">Yarn Weight</label>
                <div className="control">
                  <input className="input" type="text" placeholder="e.g., Worsted" value={yarnWeight} onChange={(e) => setYarnWeight(e.target.value)} />
                </div>
              </div>
            </div>
          </div>
          <div className="field">
            <label className="label">Description</label>
            <div className="control">
              <textarea className="textarea" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} />
            </div>
          </div>
        </div>

        {/* Instruction Groups */}
        <h2 className="title is-5">Pattern Parts</h2>
        {groups.map((group, gi) => (
          <div key={group.key} className="card mb-4">
            <div className="card-content">
              <div className="level">
                <div className="level-left">
                  <strong>Part {gi + 1}</strong>
                </div>
                {groups.length > 1 && (
                  <div className="level-right">
                    <button className="button is-small is-danger is-outlined" onClick={() => removeGroup(gi)}>
                      Remove Part
                    </button>
                  </div>
                )}
              </div>

              <div className="columns">
                <div className="column is-5">
                  <div className="field">
                    <label className="label is-small">Label</label>
                    <div className="control">
                      <input className="input is-small" type="text" placeholder={`${patternType === 'round' ? 'Round' : 'Row'} ${gi + 1}`} value={group.label} onChange={(e) => updateGroup(gi, 'label', e.target.value)} />
                    </div>
                  </div>
                </div>
                <div className="column is-2">
                  <div className="field">
                    <label className="label is-small">Repeat</label>
                    <div className="control">
                      <input className="input is-small" type="number" min={1} value={group.repeatCount} onChange={(e) => updateGroup(gi, 'repeatCount', Math.max(1, parseInt(e.target.value) || 1))} />
                    </div>
                  </div>
                </div>
                <div className="column is-2">
                  <div className="field">
                    <label className="label is-small">Expected Count</label>
                    <div className="control">
                      <input className="input is-small" type="number" min={0} value={group.expectedCount} onChange={(e) => updateGroup(gi, 'expectedCount', e.target.value)} />
                    </div>
                  </div>
                </div>
              </div>

              <div className="field">
                <label className="label is-small">Notes</label>
                <div className="control">
                  <textarea className="textarea is-small" rows={2} value={group.notes} onChange={(e) => updateGroup(gi, 'notes', e.target.value)} />
                </div>
              </div>

              {/* Stitch Entries */}
              <h3 className="title is-6 mt-4">Stitches</h3>
              <table className="table is-fullwidth is-narrow">
                <thead>
                  <tr>
                    <th>Stitch</th>
                    <th style={{ width: '100px' }}>Count</th>
                    <th style={{ width: '100px' }}>Repeat</th>
                    <th style={{ width: '60px' }}></th>
                  </tr>
                </thead>
                <tbody>
                  {group.entries.map((entry, ei) => (
                    <tr key={entry.key}>
                      <td>
                        <div className="select is-small is-fullwidth">
                          <select value={entry.stitchId} onChange={(e) => updateEntry(gi, ei, 'stitchId', Number(e.target.value))}>
                            <option value={0}>-- Select --</option>
                            {allStitches.map((s) => (
                              <option key={s.id} value={s.id}>{s.abbreviation} - {s.name}</option>
                            ))}
                          </select>
                        </div>
                      </td>
                      <td>
                        <input className="input is-small" type="number" min={1} value={entry.count} onChange={(e) => updateEntry(gi, ei, 'count', Math.max(1, parseInt(e.target.value) || 1))} />
                      </td>
                      <td>
                        <input className="input is-small" type="number" min={1} value={entry.repeatCount} onChange={(e) => updateEntry(gi, ei, 'repeatCount', Math.max(1, parseInt(e.target.value) || 1))} />
                      </td>
                      <td>
                        {group.entries.length > 1 && (
                          <button className="button is-small is-danger is-outlined" onClick={() => removeEntry(gi, ei)}>
                            &times;
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <button className="button is-small is-info is-outlined" onClick={() => addEntry(gi)}>
                + Add Stitch
              </button>
            </div>
          </div>
        ))}

        <button className="button is-info is-outlined mb-5" onClick={addGroup}>
          + Add Part
        </button>

        {/* Action Buttons */}
        <div className="buttons">
          <button className="button is-primary" onClick={() => setShowSave(true)}>
            Save Pattern
          </button>
          <button className="button is-info is-outlined" onClick={handlePreview}>
            Preview
          </button>
          <button className="button is-light" onClick={() => setShowCancel(true)}>
            Cancel
          </button>
        </div>

        {/* Save Confirmation Modal */}
        <ConfirmModal
          isOpen={showSave}
          title="Save Pattern"
          message={`Save this pattern${isEdit ? ' changes' : ''}?`}
          confirmLabel={saving ? 'Saving...' : 'Save'}
          confirmClass="is-primary"
          onConfirm={handleSave}
          onCancel={() => setShowSave(false)}
        />

        {/* Cancel Confirmation Modal */}
        <ConfirmModal
          isOpen={showCancel}
          title="Discard Changes"
          message="Are you sure you want to discard your changes?"
          confirmLabel="Discard"
          onConfirm={() => navigate('/patterns')}
          onCancel={() => setShowCancel(false)}
        />

        {/* Preview Modal */}
        {showPreview && (
          <div className="modal is-active">
            <div className="modal-background" onClick={() => setShowPreview(false)}></div>
            <div className="modal-card" style={{ maxWidth: '700px' }}>
              <header className="modal-card-head">
                <p className="modal-card-title">Pattern Preview</p>
                <button className="delete" aria-label="close" onClick={() => setShowPreview(false)}></button>
              </header>
              <section className="modal-card-body">
                <p className="mb-3"><strong>Total Stitches:</strong> {previewStitchCount}</p>
                <pre style={{ whiteSpace: 'pre-wrap', fontFamily: 'monospace', backgroundColor: '#f5f5f5', padding: '1rem', borderRadius: '4px' }}>
                  {previewText || '(empty pattern)'}
                </pre>
              </section>
              <footer className="modal-card-foot">
                <button className="button" onClick={() => setShowPreview(false)}>Close</button>
              </footer>
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
