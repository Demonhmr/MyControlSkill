import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';

// StoreProvider hydrates from localStorage on mount (persistence feature).
// jsdom keeps localStorage alive across tests within a file, so without this,
// state saved by one test (e.g. ACK_DESTRUCTOR) leaks into the next test's
// "fresh" StoreProvider and silently changes what it renders.
afterEach(() => {
  localStorage.clear();
});
