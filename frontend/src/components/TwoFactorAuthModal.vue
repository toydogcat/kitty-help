<script setup lang="ts">
import { ref } from 'vue';
import { apiService } from '../services/api';

const props = defineProps<{
    email: string;
}>();

const emit = defineEmits(['verified', 'cancel']);

const code = ref('');
const loading = ref(false);
const error = ref('');

const verify = async () => {
    if (code.value.length < 6) return;
    loading.value = true;
    error.value = '';
    try {
        const res = await apiService.verifyTOTP(code.value);
        if (res.status === 'success') {
            emit('verified', res.token);
        } else {
            error.value = '驗證失敗，請再試一次';
        }
    } catch (err: any) {
        error.value = err.response?.data?.error || '驗證碼錯誤或已過期';
    } finally {
        loading.value = false;
    }
};

const handleInput = (e: any) => {
    const val = e.target.value.replace(/\D/g, '');
    code.value = val.slice(0, 6);
    if (code.value.length === 6) {
        verify();
    }
};
</script>

<template>
    <div class="tfa-modal-overlay">
        <div class="tfa-card card glow">
            <div class="tfa-header">
                <div class="security-icon">🛡️</div>
                <h3>雙重驗證 (2FA)</h3>
                <p>偵測到新環境登入，請輸入 Google Authenticator 驗證碼</p>
                <div class="user-email">{{ email }}</div>
            </div>

            <div class="tfa-body">
                <div class="code-inputs">
                    <input 
                        type="text" 
                        v-model="code"
                        placeholder="000000"
                        maxlength="6"
                        @input="handleInput"
                        autofocus
                        class="huge-code-input"
                        :disabled="loading"
                    />
                </div>
                
                <p v-if="error" class="error-text">❌ {{ error }}</p>

                <div class="tfa-actions">
                    <button @click="verify" :disabled="loading || code.length < 6" class="primary-btn">
                        {{ loading ? '驗證中...' : '確認驗證' }}
                    </button>
                    <button @click="emit('cancel')" class="text-btn">取消登入</button>
                </div>
            </div>
            
            <div class="tfa-footer">
                <small>如果您沒有設定過 2FA 或無法登入，請聯絡系統管理員。</small>
            </div>
        </div>
    </div>
</template>

<style scoped>
.tfa-modal-overlay {
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.9);
    backdrop-filter: blur(10px);
    display: flex; align-items: center; justify-content: center;
    z-index: 10000; padding: 20px;
}

.tfa-card {
    width: 100%; max-width: 400px;
    padding: 2.5rem; text-align: center;
    background: #1a1a20;
    border: 1px solid rgba(var(--primary-rgb), 0.3);
}

.security-icon { font-size: 3rem; margin-bottom: 1rem; }
.tfa-header h3 { color: var(--primary-color); font-size: 1.5rem; margin-bottom: 0.5rem; }
.tfa-header p { font-size: 0.9rem; opacity: 0.7; margin-bottom: 0.5rem; line-height: 1.4; }
.user-email { font-weight: bold; color: var(--primary-color); margin-bottom: 2rem; }

.huge-code-input {
    width: 100%; height: 80px;
    text-align: center; font-size: 3rem;
    background: rgba(0,0,0,0.3); border: 2px solid var(--border-color);
    border-radius: 12px; color: var(--primary-color);
    font-family: 'JetBrains Mono', monospace;
    letter-spacing: 0.5rem;
    transition: all 0.3s;
}

.huge-code-input:focus { border-color: var(--primary-color); outline: none; box-shadow: 0 0 20px rgba(var(--primary-rgb), 0.3); }

.error-text { color: #ff5555; margin-top: 1rem; font-size: 0.9rem; font-weight: bold; }

.tfa-actions { display: flex; flex-direction: column; gap: 1rem; margin-top: 2rem; }
.primary-btn { 
    background: var(--primary-color); color: white; border: none; padding: 1rem; 
    border-radius: 10px; font-weight: bold; font-size: 1.1rem; cursor: pointer;
}
.text-btn { background: none; border: none; color: #888; cursor: pointer; text-decoration: underline; }

.tfa-footer { margin-top: 2rem; opacity: 0.5; font-size: 0.75rem; }
</style>
