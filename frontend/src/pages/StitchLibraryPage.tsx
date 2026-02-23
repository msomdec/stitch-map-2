import { useEffect, useState, useCallback, type FormEvent } from 'react';
import { useStitchStore } from '../stores/stitchStore';
import { ErrorNotification } from '../components/ErrorNotification';
import { ConfirmModal } from '../components/ConfirmModal';
import type { Stitch } from '../types';

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

const CATEGORIES = ['basic', 'increase', 'decrease', 'post', 'advanced', 'specialty', 'action', 'custom'];

export function StitchLibraryPage() {
  const { predefined, custom, loading, error, fetchStitches, createCustom, deleteCustom, clearError } = useStitchStore();
  const [category, setCategory] = useState('');
  const [search, setSearch] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<Stitch | null>(null);

  // Create form state
  const [newAbbr, setNewAbbr] = useState('');
  const [newName, setNewName] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [newCategory, setNewCategory] = useState('custom');

  const doFetch = useCallback(
    (cat?: string, q?: string) => {
      fetchStitches({ category: cat || undefined, search: q || undefined });
    },
    [fetchStitches],
  );

  // Only fetch on mount and when category changes (not on every search keystroke).
  useEffect(() => {
    doFetch(category, search);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- search is intentionally excluded; fetched on submit only
  }, [doFetch, category]);

  const handleFilter = (e: FormEvent) => {
    e.preventDefault();
    doFetch(category, search);
  };

  const handleClearFilters = () => {
    setCategory('');
    setSearch('');
    doFetch('', '');
  };

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await createCustom({
        abbreviation: newAbbr,
        name: newName,
        description: newDesc,
        category: newCategory,
      });
      setNewAbbr('');
      setNewName('');
      setNewDesc('');
      setNewCategory('custom');
      doFetch(category, search);
    } catch {
      // Error in store
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteCustom(deleteTarget.id);
      setDeleteTarget(null);
      doFetch(category, search);
    } catch {
      // Error in store
    }
  };

  return (
    <section className="section">
      <div className="container">
        <h1 className="title">Stitch Library</h1>

        <ErrorNotification message={error} onDismiss={clearError} />

        {/* Filter Box */}
        <div className="box mb-5">
          <form onSubmit={handleFilter}>
            <div className="field is-grouped">
              <div className="control">
                <div className="select">
                  <select value={category} onChange={(e) => setCategory(e.target.value)}>
                    <option value="">All Categories</option>
                    {CATEGORIES.map((c) => (
                      <option key={c} value={c}>{c.charAt(0).toUpperCase() + c.slice(1)}</option>
                    ))}
                  </select>
                </div>
              </div>
              <div className="control is-expanded">
                <input
                  className="input"
                  type="text"
                  placeholder="Search stitches..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <div className="control">
                <button className="button is-info" type="submit">Filter</button>
              </div>
              {(category || search) && (
                <div className="control">
                  <button className="button is-light" type="button" onClick={handleClearFilters}>
                    Clear
                  </button>
                </div>
              )}
            </div>
          </form>
        </div>

        {loading && predefined.length === 0 && (
          <div className="has-text-centered mb-4"><span className="loader"></span></div>
        )}

        {/* Standard Stitches */}
        <h2 className="title is-4">Standard Stitches</h2>
        <div className="table-container">
          <table className="table is-fullwidth is-striped is-hoverable">
            <thead>
              <tr>
                <th>Abbreviation</th>
                <th>Name</th>
                <th>Category</th>
                <th>Description</th>
              </tr>
            </thead>
            <tbody>
              {predefined.map((stitch) => (
                <tr key={stitch.id}>
                  <td><strong>{stitch.abbreviation}</strong></td>
                  <td>{stitch.name}</td>
                  <td><span className={`tag ${categoryTagClass(stitch.category)}`}>{stitch.category}</span></td>
                  <td>{stitch.description}</td>
                </tr>
              ))}
              {!loading && predefined.length === 0 && (
                <tr><td colSpan={4} className="has-text-centered has-text-grey">No stitches match your filters</td></tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Custom Stitches */}
        <h2 className="title is-4 mt-6">Custom Stitches</h2>

        {/* Add Form */}
        <div className="box mb-4">
          <h3 className="title is-6">Add Custom Stitch</h3>
          <form onSubmit={handleCreate}>
            <div className="columns">
              <div className="column is-2">
                <div className="field">
                  <label className="label is-small">Abbreviation</label>
                  <div className="control">
                    <input
                      className="input is-small"
                      type="text"
                      value={newAbbr}
                      onChange={(e) => setNewAbbr(e.target.value)}
                      required
                    />
                  </div>
                </div>
              </div>
              <div className="column is-3">
                <div className="field">
                  <label className="label is-small">Name</label>
                  <div className="control">
                    <input
                      className="input is-small"
                      type="text"
                      value={newName}
                      onChange={(e) => setNewName(e.target.value)}
                      required
                    />
                  </div>
                </div>
              </div>
              <div className="column is-2">
                <div className="field">
                  <label className="label is-small">Category</label>
                  <div className="control">
                    <div className="select is-small is-fullwidth">
                      <select value={newCategory} onChange={(e) => setNewCategory(e.target.value)}>
                        <option value="custom">Custom</option>
                        <option value="basic">Basic</option>
                        <option value="increase">Increase</option>
                        <option value="decrease">Decrease</option>
                        <option value="advanced">Advanced</option>
                        <option value="specialty">Specialty</option>
                        <option value="action">Action</option>
                      </select>
                    </div>
                  </div>
                </div>
              </div>
              <div className="column is-3">
                <div className="field">
                  <label className="label is-small">Description</label>
                  <div className="control">
                    <input
                      className="input is-small"
                      type="text"
                      value={newDesc}
                      onChange={(e) => setNewDesc(e.target.value)}
                    />
                  </div>
                </div>
              </div>
              <div className="column is-2">
                <div className="field">
                  <label className="label is-small">&nbsp;</label>
                  <div className="control">
                    <button className="button is-primary is-small is-fullwidth" type="submit">
                      Add Stitch
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </form>
        </div>

        {custom.length === 0 ? (
          <div className="notification is-light">No custom stitches yet. Add one above!</div>
        ) : (
          <div className="table-container">
            <table className="table is-fullwidth is-striped is-hoverable">
              <thead>
                <tr>
                  <th>Abbreviation</th>
                  <th>Name</th>
                  <th>Category</th>
                  <th>Description</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {custom.map((stitch) => (
                  <tr key={stitch.id}>
                    <td><strong>{stitch.abbreviation}</strong></td>
                    <td>{stitch.name}</td>
                    <td><span className={`tag ${categoryTagClass(stitch.category)}`}>{stitch.category}</span></td>
                    <td>{stitch.description}</td>
                    <td>
                      <button
                        className="button is-small is-danger is-outlined"
                        onClick={() => setDeleteTarget(stitch)}
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <ConfirmModal
          isOpen={!!deleteTarget}
          title="Delete Custom Stitch"
          message={`Are you sure you want to delete "${deleteTarget?.name}" (${deleteTarget?.abbreviation})?`}
          onConfirm={handleDelete}
          onCancel={() => setDeleteTarget(null)}
        />
      </div>
    </section>
  );
}
