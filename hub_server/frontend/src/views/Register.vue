<template>
  <div class="min-h-screen bg-bg flex items-center justify-center p-4">
    <div class="bg-white rounded-3xl shadow-neu p-8 w-full max-w-md border border-slate-100">
      <div class="text-center mb-10">
        <h1 class="text-3xl font-serif font-bold text-slate-900 mb-2">创建账号</h1>
        <p class="text-slate-500">加入 WX Channel Hub 开始您的分布式任务</p>
      </div>

      <form @submit.prevent="handleRegister" class="space-y-6">
        <div>
          <label class="block text-sm font-bold text-slate-700 mb-2 uppercase tracking-wider">邮箱</label>
          <input 
            v-model="email"
            type="email" 
            required
            class="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all text-slate-800"
            placeholder="you@example.com"
          >
        </div>

        <div>
           <label class="block text-sm font-bold text-slate-700 mb-2 uppercase tracking-wider">密码</label>
          <input 
            v-model="password"
            type="password" 
            required
            minlength="6"
            class="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all text-slate-800"
            placeholder="••••••••"
          >
        </div>

        <div>
           <label class="block text-sm font-bold text-slate-700 mb-2 uppercase tracking-wider">确认密码</label>
          <input 
            v-model="confirmPassword"
            type="password" 
            required
            minlength="6"
            class="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all text-slate-800"
            placeholder="••••••••"
          >
        </div>

        <div v-if="error" class="bg-red-50 text-red-600 px-4 py-3 rounded-xl text-sm font-medium">
          {{ error }}
        </div>
        <div v-if="userStore.error" class="bg-red-50 text-red-600 px-4 py-3 rounded-xl text-sm font-medium">
          {{ userStore.error }}
        </div>

        <button 
          type="submit"
          :disabled="userStore.loading"
          class="w-full py-4 rounded-xl bg-slate-900 text-white font-bold text-lg shadow-lg shadow-slate-200 transition-all hover:-translate-y-1 hover:shadow-xl disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <span v-if="userStore.loading">注册中...</span>
          <span v-else>注册</span>
        </button>
      </form>

      <div class="mt-8 text-center text-sm text-slate-500 font-medium">
        已有账号? 
        <router-link to="/login" class="text-primary hover:text-primary-dark ml-1 font-bold underline decoration-2 decoration-transparent hover:decoration-current transition-all">
          立即登录
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../store/user'

const email = ref('')
const password = ref('')
const confirmPassword = ref('')
const error = ref('')
const userStore = useUserStore()
const router = useRouter()

const handleRegister = async () => {
  error.value = ''
  if (password.value !== confirmPassword.value) {
    error.value = '两次输入的密码不一致'
    return
  }
  
  const success = await userStore.register(email.value, password.value)
  if (success) {
    router.push('/')
  }
}
</script>
