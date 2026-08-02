import { useStore, hasCriticalDestructor } from '../state/store.jsx';
import { useProfile } from '../state/useProfile.js';
import { MIN_RESPONDENTS } from '../logic/scoring.js';
import { Card, Banner, Badge, Button } from '../components/ui.jsx';

export default function DestructorsScreen() {
  const { state, dispatch } = useStore();
  const profile = useProfile();
  const sorted = [...profile.destructors].sort((a, b) => a.percentile - b.percentile);
  const critical = sorted.filter((d) => d.percentile < 10);
  const blocked = hasCriticalDestructor(state, profile.destructors);

  return (
    <section>
      <h1 className="scr-title">Критические зоны (деструкторы)</h1>
      <p className="scr-sub">
        Компетенции ниже 10-го перцентиля нивелируют эффект всех сильных сторон. Пока критическая зона
        не проработана — остальные разделы приложения доступны только для просмотра.
      </p>

      {!profile.ready && (
        <Banner title="Показаны демонстрационные значения">
          Собрано {profile.respondentCount} из {MIN_RESPONDENTS} необходимых внешних ответов 360°. Заполните
          анкеты на экране «Опрос 360°», чтобы увидеть реальный расчёт.
        </Banner>
      )}

      {blocked ? (
        <Banner title={`Обнаружена критическая зона — ${critical.length}`}>
          Пока она активна, разделы «Точка роста» и «План развития» заблокированы для выбора. Это единственный приоритет.
        </Banner>
      ) : (
        <Banner tone="ok" title="Критических зон нет">
          Можно переходить к поиску сильных сторон и точки роста.
        </Banner>
      )}

      <Card>
        <div className="sec-label">10 деструкторов, отсортировано от самого критичного</div>
        {sorted.map((d) => {
          const isCrit = d.percentile < 10 && !state.destructorAcknowledged;
          return (
            <div key={d.id}>
              <div className={`destructor-row ${isCrit ? 'crit' : ''}`}>
                <Badge tone={isCrit ? 'critical' : 'neutral'}>{isCrit ? 'Критично' : 'Норма'}</Badge>
                <span className="name">{d.name}</span>
                <span className="pct">{d.percentile}-й перц.{!d.isLive ? ' (демо)' : ''}</span>
              </div>
              {d.percentile < 10 && (
                <div className="destructor-detail">
                  {d.quote && <div style={{ marginBottom: 8 }}>{d.quote}</div>}
                  <div>
                    {state.destructorAcknowledged
                      ? 'Отмечено как проработанное. Следующий пульс-чек подтвердит сдвиг перцентиля выше 10.'
                      : 'Это единственный приоритет прямо сейчас. Начните с тренажёра по этой зоне или подтвердите, что она уже в работе.'}
                  </div>
                  <div className="btn-row">
                    <Button
                      variant="secondary"
                      onClick={() => {
                        dispatch({ type: 'SET_TRAINER_SCENARIO', code: 'DESTRUCTOR_VIS' });
                        dispatch({ type: 'SET_SCREEN', screen: 'trainer' });
                      }}
                    >
                      Открыть тренажёр по этой зоне →
                    </Button>
                    {!state.destructorAcknowledged && (
                      <Button variant="ghost" onClick={() => dispatch({ type: 'ACK_DESTRUCTOR' })}>
                        Отметить как проработанное (демо)
                      </Button>
                    )}
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </Card>

      <Card>
        <div className="muted" style={{ fontSize: 12.5 }}>
          Каждому деструктору соответствует поведенческий вопрос в 360°-опросе. Значение — перцентиль частоты
          негативного поведения по оценке окружения (не самооценка).
        </div>
      </Card>
    </section>
  );
}
