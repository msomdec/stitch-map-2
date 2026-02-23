import { useEffect } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { useAuthStore } from './stores/authStore';
import { Layout } from './components/Layout';
import { ProtectedRoute } from './components/ProtectedRoute';
import { HomePage } from './pages/HomePage';
import { LoginPage } from './pages/LoginPage';
import { RegisterPage } from './pages/RegisterPage';
import { DashboardPage } from './pages/DashboardPage';
import { PatternListPage } from './pages/PatternListPage';
import { PatternEditorPage } from './pages/PatternEditorPage';
import { PatternViewPage } from './pages/PatternViewPage';
import { StitchLibraryPage } from './pages/StitchLibraryPage';
import { WorkSessionPage } from './pages/WorkSessionPage';
import { NotFoundPage } from './pages/NotFoundPage';

function App() {
  const fetchMe = useAuthStore((s) => s.fetchMe);

  useEffect(() => {
    fetchMe();
  }, [fetchMe]);

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<HomePage />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <DashboardPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/patterns"
            element={
              <ProtectedRoute>
                <PatternListPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/patterns/new"
            element={
              <ProtectedRoute>
                <PatternEditorPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/patterns/:id"
            element={
              <ProtectedRoute>
                <PatternViewPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/patterns/:id/edit"
            element={
              <ProtectedRoute>
                <PatternEditorPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/stitches"
            element={
              <ProtectedRoute>
                <StitchLibraryPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/sessions/:id"
            element={
              <ProtectedRoute>
                <WorkSessionPage />
              </ProtectedRoute>
            }
          />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
