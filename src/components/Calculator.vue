<script setup lang="ts">
import { ref } from 'vue';

const display = ref('0');
const prevValue = ref<number | null>(null);
const operator = ref<string | null>(null);
const resetDisplay = ref(false);

const appendNumber = (num: string) => {
  if (display.value === '0' || resetDisplay.value) {
    display.value = num;
    resetDisplay.value = false;
  } else {
    display.value += num;
  }
};

const setOperator = (op: string) => {
  if (operator.value && !resetDisplay.value) {
    calculate();
  }
  prevValue.value = parseFloat(display.value);
  operator.value = op;
  resetDisplay.value = true;
};

const calculate = () => {
  if (operator.value === null || prevValue.value === null) return;
  const current = parseFloat(display.value);
  let result = 0;
  
  switch (operator.value) {
    case '+': result = prevValue.value + current; break;
    case '-': result = prevValue.value - current; break;
    case '*': result = prevValue.value * current; break;
    case '/': result = current !== 0 ? prevValue.value / current : 0; break;
  }
  
  display.value = Number(result.toFixed(8)).toString();
  operator.value = null;
  prevValue.value = null;
  resetDisplay.value = true;
};

const clear = () => {
  display.value = '0';
  prevValue.value = null;
  operator.value = null;
  resetDisplay.value = false;
};
</script>

<template>
  <div class="calculator card">
    <div class="calc-header">
      <span class="calc-icon">🧮</span>
      <h3>Calculator</h3>
    </div>
    
    <div class="display">
      <div class="display-value">{{ display }}</div>
    </div>
    
    <div class="keypad">
      <button @click="clear" class="btn btn-clear">AC</button>
      <button @click="setOperator('/')" class="btn btn-op">÷</button>
      
      <button @click="appendNumber('7')" class="btn">7</button>
      <button @click="appendNumber('8')" class="btn">8</button>
      <button @click="appendNumber('9')" class="btn">9</button>
      <button @click="setOperator('*')" class="btn btn-op">×</button>
      
      <button @click="appendNumber('4')" class="btn">4</button>
      <button @click="appendNumber('5')" class="btn">5</button>
      <button @click="appendNumber('6')" class="btn">6</button>
      <button @click="setOperator('-')" class="btn btn-op">−</button>
      
      <button @click="appendNumber('1')" class="btn">1</button>
      <button @click="appendNumber('2')" class="btn">2</button>
      <button @click="appendNumber('3')" class="btn">3</button>
      <button @click="setOperator('+')" class="btn btn-op">+</button>
      
      <button @click="appendNumber('0')" class="btn btn-zero">0</button>
      <button @click="appendNumber('.')" class="btn">.</button>
      <button @click="calculate" class="btn btn-equal">=</button>
    </div>
  </div>
</template>

<style scoped>
.calculator {
  max-width: 300px;
  background: linear-gradient(135deg, var(--card-bg), rgba(255, 255, 255, 0.05));
  border: 1px solid var(--border-color);
  padding: 1.5rem;
  border-radius: 20px;
  box-shadow: 0 10px 30px rgba(0,0,0,0.3);
}

.calc-header {
  display: flex;
  align-items: center;
  gap: 0.8rem;
  margin-bottom: 1rem;
}

.calc-icon { font-size: 1.2rem; }

.display {
  background: rgba(0, 0, 0, 0.3);
  padding: 1rem;
  border-radius: 12px;
  margin-bottom: 1.2rem;
  text-align: right;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.display-value {
  font-size: 1.8rem;
  font-family: 'JetBrains Mono', monospace;
  color: var(--primary-color);
  overflow: hidden;
  text-overflow: ellipsis;
}

.keypad {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 0.6rem;
}

.btn {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-color);
  color: var(--text-color);
  padding: 0.8rem;
  border-radius: 10px;
  font-size: 1.1rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.btn:hover {
  background: rgba(255, 255, 255, 0.1);
  transform: translateY(-2px);
}

.btn:active { transform: translateY(0); }

.btn-op {
  color: var(--primary-color);
  background: rgba(var(--primary-rgb), 0.1);
}

.btn-clear {
  grid-column: span 3;
  color: #ef4444;
}

.btn-zero {
  grid-column: span 2;
}

.btn-equal {
  background: var(--primary-color);
  color: white;
  border: none;
}

.btn-equal:hover {
  background: var(--primary-color);
  filter: brightness(1.2);
  box-shadow: 0 4px 15px rgba(var(--primary-rgb), 0.4);
}
</style>
