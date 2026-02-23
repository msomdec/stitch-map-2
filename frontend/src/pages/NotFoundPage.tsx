import { Link } from 'react-router-dom';

export function NotFoundPage() {
  return (
    <section className="section">
      <div className="container has-text-centered">
        <h1 className="title is-1 has-text-grey-light">404</h1>
        <h2 className="title is-4">Page Not Found</h2>
        <p className="mb-4">The page you're looking for doesn't exist.</p>
        <Link className="button is-primary" to="/">
          Go Home
        </Link>
      </div>
    </section>
  );
}
