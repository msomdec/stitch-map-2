const API_BASE = '/api';

export class ApiClientError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiClientError';
    this.status = status;
  }
}

// Callback invoked on 401 responses to clear stale auth state.
// Set by the auth store on initialization.
let onUnauthorized: (() => void) | null = null;

export function setOnUnauthorized(callback: () => void) {
  onUnauthorized = callback;
}

async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    // Clear stale auth on 401, but not for auth endpoints themselves
    // (login/register 401s are expected validation errors).
    if (response.status === 401 && onUnauthorized) {
      const isAuthEndpoint = response.url.includes('/auth/login') || response.url.includes('/auth/register');
      if (!isAuthEndpoint) {
        onUnauthorized();
      }
    }
    let message = response.statusText;
    try {
      const body = await response.json();
      if (body.error) message = body.error;
    } catch { /* ignore parse errors */ }
    throw new ApiClientError(response.status, message);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(url, { credentials: 'include', ...init });
  } catch (err) {
    // Let AbortController cancellations propagate as-is
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw err;
    }
    // Network failure, DNS error, offline, etc.
    throw new ApiClientError(0, 'Network error. Please check your connection and try again.');
  }
  return handleResponse<T>(response);
}

export async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  return request<T>(`${API_BASE}${path}`, { signal });
}

export async function post<T>(path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  return request<T>(`${API_BASE}${path}`, {
    method: 'POST',
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
    signal,
  });
}

export async function put<T>(path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  return request<T>(`${API_BASE}${path}`, {
    method: 'PUT',
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
    signal,
  });
}

export async function del<T>(path: string, signal?: AbortSignal): Promise<T> {
  return request<T>(`${API_BASE}${path}`, {
    method: 'DELETE',
    signal,
  });
}

export async function uploadFile<T>(path: string, file: File, fieldName = 'image'): Promise<T> {
  const formData = new FormData();
  formData.append(fieldName, file);
  return request<T>(`${API_BASE}${path}`, {
    method: 'POST',
    body: formData,
  });
}
