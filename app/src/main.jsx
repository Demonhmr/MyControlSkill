import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App.jsx';
import RespondentSurveyScreen from './screens/RespondentSurveyScreen.jsx';
import './styles/theme.css';

// Полноценного роутера в приложении нет: экранами управляет стор. Но анкета
// респондента — не экран приложения, а отдельный вход по ссылке из письма,
// без навигации и без доступа к профилю руководителя. Поэтому разбор пути
// здесь, до создания стора.
const surveyMatch = window.location.pathname.match(/^\/s\/(.+)$/);

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    {surveyMatch ? (
      <RespondentSurveyScreen token={decodeURIComponent(surveyMatch[1])} />
    ) : (
      <App />
    )}
  </React.StrictMode>
);
