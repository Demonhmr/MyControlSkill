import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { StoreProvider } from '../state/store.jsx';
import DestructorsScreen from './DestructorsScreen.jsx';
import GrowthPointScreen from './GrowthPointScreen.jsx';

// Свежее состояние по умолчанию содержит демо-деструктор с перцентилем 8 (< 10),
// поэтому GrowthPointScreen заблокирован, пока его не "проработают" на DestructorsScreen.
function renderBoth() {
  return render(
    <StoreProvider>
      <DestructorsScreen />
      <GrowthPointScreen />
    </StoreProvider>
  );
}

describe('GrowthPointScreen', () => {
  it('заблокирован, пока не проработана критическая зона', () => {
    renderBoth();
    expect(screen.getByText('Раздел заблокирован')).toBeInTheDocument();
  });

  it('разблокируется после подтверждения деструктора, кнопка недоступна без интереса', async () => {
    const user = userEvent.setup();
    renderBoth();
    await user.click(screen.getByRole('button', { name: 'Отметить как проработанное (демо)' }));
    const buttons = screen.getAllByRole('button', { name: 'Выбрать точкой роста' });
    buttons.forEach((btn) => expect(btn).toBeDisabled());
  });

  it('кнопка становится доступна после отметки интереса', async () => {
    const user = userEvent.setup();
    renderBoth();
    await user.click(screen.getByRole('button', { name: 'Отметить как проработанное (демо)' }));
    const checkboxes = screen.getAllByRole('checkbox');
    await user.click(checkboxes[0]);
    const buttons = screen.getAllByRole('button', { name: 'Выбрать точкой роста' });
    expect(buttons[0]).toBeEnabled();
  });
});
