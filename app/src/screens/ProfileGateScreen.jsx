import { useStore } from '../state/store.jsx';
import { useProfileSource } from '../state/profile.jsx';
import { Card, Banner, Button } from '../components/ui.jsx';

/**
 * Заглушка вместо экранов с числами, пока чисел нет.
 *
 * В сетевом режиме показывать демонстрационные значения нельзя: у них та же
 * форма, что у настоящих, и руководитель принял бы витрину прототипа за
 * результат замера своей команды.
 */
export default function ProfileGateScreen() {
  const { dispatch } = useStore();
  const { status, counts, error, reload } = useProfileSource();
  const goToRounds = () => dispatch({ type: 'SET_SCREEN', screen: 'rounds' });

  if (status === 'loading') {
    return (
      <section>
        <h1 className="scr-title">Профиль</h1>
        <Card>Загружаем профиль…</Card>
      </section>
    );
  }

  if (status === 'error') {
    return (
      <section>
        <h1 className="scr-title">Профиль</h1>
        <Banner title="Не удалось загрузить профиль">
          {error?.status === 401
            ? 'Сессия истекла — войдите заново.'
            : 'Попробуйте обновить чуть позже.'}
        </Banner>
        <Card>
          <div className="btn-row">
            <Button onClick={reload}>Повторить</Button>
          </div>
        </Card>
      </section>
    );
  }

  if (status === 'no-assessments') {
    return (
      <section>
        <h1 className="scr-title">Профиль</h1>
        <Banner title="Раунд ещё не создан">
          Профиль строится по анкетам 360°. Создайте раунд и разошлите ссылки коллегам.
        </Banner>
        <Card>
          <div className="btn-row">
            <Button onClick={goToRounds}>Перейти к раундам</Button>
          </div>
        </Card>
      </section>
    );
  }

  const collected = counts?.external ?? 0;
  const required = counts?.required ?? 3;

  return (
    <section>
      <h1 className="scr-title">Профиль</h1>
      <Banner title={`Собрано ${collected} из ${required} анкет`}>
        Расчёт начнётся, когда ответят как минимум {required} человека. Это не техническое
        ограничение: на меньшем числе анкет перцентили описывают случайность, а не руководителя.
        Демонстрационных чисел здесь нет намеренно — их легко принять за результат замера.
      </Banner>
      <Card>
        <div className="btn-row">
          <Button onClick={goToRounds}>Пригласить ещё</Button>
          <Button variant="ghost" onClick={reload}>Обновить</Button>
        </div>
      </Card>
    </section>
  );
}
