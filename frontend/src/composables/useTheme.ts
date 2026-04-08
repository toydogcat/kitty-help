import { ref, onMounted } from 'vue';
import Cookies from 'js-cookie';

export type Theme = 'futurist' | 'cyberpunk' | 'forest' | 'ocean' | 'sunset';

const THEME_COOKIE_KEY = 'kitty-help-theme';

export function useTheme() {
  const currentTheme = ref<Theme>('futurist');

  const setTheme = (theme: Theme) => {
    currentTheme.value = theme;
    Cookies.set(THEME_COOKIE_KEY, theme, { expires: 365 });
    applyThemeToBody(theme);
  };

  const applyThemeToBody = (theme: Theme) => {
    const body = document.body;
    body.classList.remove('theme-modern', 'theme-retro', 'theme-futurist', 'theme-cyberpunk', 'theme-forest', 'theme-ocean', 'theme-sunset');
    body.classList.add(`theme-${theme}`);
  };

  onMounted(() => {
    const savedTheme = Cookies.get(THEME_COOKIE_KEY) as Theme;
    const validThemes: Theme[] = ['futurist', 'cyberpunk', 'forest', 'ocean', 'sunset'];
    if (savedTheme && validThemes.includes(savedTheme)) {
      setTheme(savedTheme);
    } else {
      setTheme('futurist');
    }
  });

  return {
    currentTheme,
    setTheme,
  };
}
