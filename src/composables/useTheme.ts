import { ref, onMounted } from 'vue';
import Cookies from 'js-cookie';

export type Theme = 'modern' | 'retro' | 'futurist';

const THEME_COOKIE_KEY = 'kitty-help-theme';

export function useTheme() {
  const currentTheme = ref<Theme>('modern');

  const setTheme = (theme: Theme) => {
    currentTheme.value = theme;
    Cookies.set(THEME_COOKIE_KEY, theme, { expires: 365 });
    applyThemeToBody(theme);
  };

  const applyThemeToBody = (theme: Theme) => {
    const body = document.body;
    body.classList.remove('theme-modern', 'theme-retro', 'theme-futurist');
    body.classList.add(`theme-${theme}`);
  };

  onMounted(() => {
    const savedTheme = Cookies.get(THEME_COOKIE_KEY) as Theme;
    if (savedTheme && ['modern', 'retro', 'futurist'].includes(savedTheme)) {
      setTheme(savedTheme);
    } else {
      setTheme('modern');
    }
  });

  return {
    currentTheme,
    setTheme,
  };
}
