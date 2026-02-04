<template>
  <div class="w-full space-y-8 p-8">
    <!-- Stats Cards -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
       <!-- Users -->
       <div class="bg-white rounded-3xl p-6 shadow-card border border-slate-100 flex items-center justify-between">
           <div>
               <p class="text-slate-500 text-sm font-medium mb-1">总用户数</p>
               <h3 class="text-3xl font-bold text-slate-800">{{ stats.users || 0 }}</h3>
           </div>
           <div class="w-12 h-12 rounded-2xl bg-blue-50 text-blue-500 flex items-center justify-center">
               <component :is="Users" class="w-6 h-6" />
           </div>
       </div>
       
       <!-- Devices -->
       <div class="bg-white rounded-3xl p-6 shadow-card border border-slate-100 flex items-center justify-between">
           <div>
               <p class="text-slate-500 text-sm font-medium mb-1">活跃设备</p>
               <h3 class="text-3xl font-bold text-slate-800">{{ stats.devices || 0 }}</h3>
           </div>
           <div class="w-12 h-12 rounded-2xl bg-purple-50 text-purple-500 flex items-center justify-center">
               <component :is="Monitor" class="w-6 h-6" />
           </div>
       </div>

       <!-- Transactions -->
       <div class="bg-white rounded-3xl p-6 shadow-card border border-slate-100 flex items-center justify-between">
           <div>
               <p class="text-slate-500 text-sm font-medium mb-1">交易记录</p>
               <h3 class="text-3xl font-bold text-slate-800">{{ stats.transactions || 0 }}</h3>
           </div>
           <div class="w-12 h-12 rounded-2xl bg-green-50 text-green-500 flex items-center justify-center">
               <component :is="Receipt" class="w-6 h-6" />
           </div>
       </div>

       <!-- Total Credits -->
       <div class="bg-white rounded-3xl p-6 shadow-card border border-slate-100 flex items-center justify-between">
           <div>
               <p class="text-slate-500 text-sm font-medium mb-1">积分流通量</p>
               <h3 class="text-3xl font-bold text-amber-500">{{ stats.total_credits || 0 }}</h3>
           </div>
           <div class="w-12 h-12 rounded-2xl bg-amber-50 text-amber-500 flex items-center justify-center">
               <component :is="Coins" class="w-6 h-6" />
           </div>
       </div>
    </div>

    <!-- User Table -->
    <div class="bg-white rounded-3xl p-8 shadow-card border border-slate-100">
      <div class="flex items-center justify-between mb-6">
        <h2 class="text-xl font-bold text-slate-800">用户列表</h2>
        <button 
          @click="fetchData"
          class="px-4 py-2 bg-bg shadow-neu rounded-xl text-slate-600 hover:shadow-neu-sm transition-all flex items-center gap-2"
        >
          <component :is="RefreshCw" class="w-4 h-4" :class="{ 'animate-spin': loading }" />
          刷新
        </button>
      </div>
      
      <div v-if="loading" class="text-center py-10 text-slate-400">
          加载中...
      </div>

      <div v-else class="overflow-x-auto">
          <table class="w-full text-left border-collapse">
              <thead>
                  <tr>
                      <th class="p-4 border-b border-slate-100 text-slate-400 font-medium text-sm">ID</th>
                      <th class="p-4 border-b border-slate-100 text-slate-400 font-medium text-sm">用户邮箱</th>
                      <th class="p-4 border-b border-slate-100 text-slate-400 font-medium text-sm">角色</th>
                      <th class="p-4 border-b border-slate-100 text-slate-400 font-medium text-sm">当前积分</th>
                      <th class="p-4 border-b border-slate-100 text-slate-400 font-medium text-sm">注册时间</th>
                      <th class="p-4 border-b border-slate-100 text-slate-400 font-medium text-sm">操作</th>
                  </tr>
              </thead>
              <tbody>
                  <tr v-for="user in users" :key="user.id" class="group hover:bg-slate-50 transition-colors">
                      <td class="p-4 border-b border-slate-100 text-slate-500 font-mono text-xs">#{{ user.id }}</td>
                      <td class="p-4 border-b border-slate-100 font-medium text-slate-700">{{ user.email }}</td>
                      <td class="p-4 border-b border-slate-100">
                          <span :class="user.role === 'admin' ? 'bg-purple-100 text-purple-700' : 'bg-slate-100 text-slate-600'" class="px-2 py-1 rounded-md text-xs font-bold uppercase">{{ user.role }}</span>
                      </td>
                      <td class="p-4 border-b border-slate-100 font-mono font-bold text-amber-600">{{ user.credits }}</td>
                      <td class="p-4 border-b border-slate-100 text-slate-400 text-sm">{{ formatDate(user.created_at) }}</td>
                      <td class="p-4 border-b border-slate-100">
                        <div class="flex gap-2">
                          <button
                            @click="openEditCredits(user)"
                            class="px-3 py-1 bg-bg shadow-neu rounded-lg text-amber-600 hover:shadow-neu-sm transition-all flex items-center gap-1 text-sm"
                            title="编辑积分"
                          >
                            <component :is="Coins" class="w-3 h-3" />
                            积分
                          </button>
                          <button
                            @click="openEditRole(user)"
                            class="px-3 py-1 bg-bg shadow-neu rounded-lg text-purple-600 hover:shadow-neu-sm transition-all flex items-center gap-1 text-sm"
                            title="修改角色"
                          >
                            <component :is="Shield" class="w-3 h-3" />
                            角色
                          </button>
                          <button
                            @click="confirmDeleteUser(user)"
                            class="px-3 py-1 bg-bg shadow-neu rounded-lg text-red-600 hover:shadow-neu-sm transition-all flex items-center gap-1 text-sm"
                            title="删除用户"
                          >
                            <component :is="Trash2" class="w-3 h-3" />
                            删除
                          </button>
                        </div>
                      </td>
                  </tr>
              </tbody>
          </table>
      </div>
    </div>

    <!-- 编辑积分对话框 -->
    <div 
      v-if="showEditCredits"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      @click.self="showEditCredits = false"
    >
      <div class="bg-white shadow-card rounded-2xl p-8 max-w-md w-full mx-4">
        <div class="flex items-center gap-4 mb-6">
          <div class="w-12 h-12 rounded-xl bg-amber-100 flex items-center justify-center">
            <component :is="Coins" class="w-6 h-6 text-amber-600" />
          </div>
          <div>
            <h3 class="text-xl font-bold text-slate-800">编辑积分</h3>
            <p class="text-sm text-slate-500">{{ selectedUser?.email }}</p>
          </div>
        </div>

        <div class="mb-6">
          <label class="block text-sm font-medium text-slate-700 mb-2">当前积分</label>
          <p class="text-2xl font-bold text-amber-600 mb-4">{{ selectedUser?.credits }}</p>
          
          <label class="block text-sm font-medium text-slate-700 mb-2">调整积分</label>
          <input
            v-model.number="creditsAdjustment"
            type="number"
            class="w-full px-4 py-2 border border-slate-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary"
            placeholder="输入正数增加，负数减少"
          />
          <p class="text-xs text-slate-500 mt-2">
            调整后积分：{{ (selectedUser?.credits || 0) + (creditsAdjustment || 0) }}
          </p>
        </div>

        <div class="flex gap-3">
          <button
            @click="showEditCredits = false"
            class="flex-1 px-4 py-3 bg-bg shadow-neu rounded-xl text-slate-600 hover:shadow-neu-sm transition-all"
          >
            取消
          </button>
          <button
            @click="updateCredits"
            :disabled="actionLoading"
            class="flex-1 px-4 py-3 rounded-xl bg-amber-600 text-white hover:bg-amber-700 transition-all"
          >
            {{ actionLoading ? '处理中...' : '确认' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 修改角色对话框 -->
    <div 
      v-if="showEditRole"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      @click.self="showEditRole = false"
    >
      <div class="bg-white shadow-card rounded-2xl p-8 max-w-md w-full mx-4">
        <div class="flex items-center gap-4 mb-6">
          <div class="w-12 h-12 rounded-xl bg-purple-100 flex items-center justify-center">
            <component :is="Shield" class="w-6 h-6 text-purple-600" />
          </div>
          <div>
            <h3 class="text-xl font-bold text-slate-800">修改角色</h3>
            <p class="text-sm text-slate-500">{{ selectedUser?.email }}</p>
          </div>
        </div>

        <div class="mb-6">
          <label class="block text-sm font-medium text-slate-700 mb-2">当前角色</label>
          <p class="text-lg font-bold text-purple-600 mb-4 uppercase">{{ selectedUser?.role }}</p>
          
          <label class="block text-sm font-medium text-slate-700 mb-2">新角色</label>
          <select
            v-model="newRole"
            class="w-full px-4 py-2 border border-slate-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary"
          >
            <option value="user">User（普通用户）</option>
            <option value="admin">Admin（管理员）</option>
          </select>
        </div>

        <div class="flex gap-3">
          <button
            @click="showEditRole = false"
            class="flex-1 px-4 py-3 bg-bg shadow-neu rounded-xl text-slate-600 hover:shadow-neu-sm transition-all"
          >
            取消
          </button>
          <button
            @click="updateRole"
            :disabled="actionLoading"
            class="flex-1 px-4 py-3 rounded-xl bg-purple-600 text-white hover:bg-purple-700 transition-all"
          >
            {{ actionLoading ? '处理中...' : '确认' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 删除用户确认对话框 -->
    <div 
      v-if="showDeleteConfirm"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
      @click.self="showDeleteConfirm = false"
    >
      <div class="bg-white shadow-card rounded-2xl p-8 max-w-md w-full mx-4">
        <div class="flex items-center gap-4 mb-6">
          <div class="w-12 h-12 rounded-xl bg-red-100 flex items-center justify-center">
            <component :is="Trash2" class="w-6 h-6 text-red-600" />
          </div>
          <div>
            <h3 class="text-xl font-bold text-slate-800">删除用户</h3>
            <p class="text-sm text-slate-500">此操作不可恢复</p>
          </div>
        </div>

        <div class="mb-6">
          <p class="text-slate-700 mb-4">
            确定要删除用户 <span class="font-bold">{{ selectedUser?.email }}</span> 吗？
          </p>
          <div class="bg-red-50 rounded-xl p-4">
            <p class="text-sm text-red-600">
              ⚠️ 删除后，该用户的所有数据（设备、订阅、任务等）都将被永久删除。
            </p>
          </div>
        </div>

        <div class="flex gap-3">
          <button
            @click="showDeleteConfirm = false"
            class="flex-1 px-4 py-3 bg-bg shadow-neu rounded-xl text-slate-600 hover:shadow-neu-sm transition-all"
          >
            取消
          </button>
          <button
            @click="deleteUser"
            :disabled="actionLoading"
            class="flex-1 px-4 py-3 rounded-xl bg-red-600 text-white hover:bg-red-700 transition-all"
          >
            {{ actionLoading ? '删除中...' : '确认删除' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { useRouter } from 'vue-router'
import { Users, Monitor, Receipt, Coins, RefreshCw, Shield, Trash2 } from 'lucide-vue-next'

const stats = ref({})
const users = ref([])
const loading = ref(true)
const router = useRouter()

// 对话框状态
const showEditCredits = ref(false)
const showEditRole = ref(false)
const showDeleteConfirm = ref(false)
const selectedUser = ref(null)
const actionLoading = ref(false)

// 表单数据
const creditsAdjustment = ref(0)
const newRole = ref('user')

const fetchData = async () => {
    loading.value = true
    try {
        const [statsRes, usersRes] = await Promise.all([
            axios.get('/api/admin/stats'),
            axios.get('/api/admin/users')
        ])
        stats.value = statsRes.data
        users.value = usersRes.data.list
    } catch (err) {
        if (err.response && err.response.status === 403) {
            alert("需要管理员权限")
            router.push('/dashboard')
        }
    } finally {
        loading.value = false
    }
}

const formatDate = (dateStr) => {
    return new Date(dateStr).toLocaleDateString()
}

// 打开编辑积分对话框
const openEditCredits = (user) => {
    selectedUser.value = user
    creditsAdjustment.value = 0
    showEditCredits.value = true
}

// 打开修改角色对话框
const openEditRole = (user) => {
    selectedUser.value = user
    newRole.value = user.role
    showEditRole.value = true
}

// 打开删除确认对话框
const confirmDeleteUser = (user) => {
    selectedUser.value = user
    showDeleteConfirm.value = true
}

// 更新积分
const updateCredits = async () => {
    if (!selectedUser.value || creditsAdjustment.value === 0) {
        alert('请输入调整金额')
        return
    }

    actionLoading.value = true
    try {
        await axios.post('/api/admin/user/credits', {
            user_id: selectedUser.value.id,
            adjustment: creditsAdjustment.value
        })
        
        showEditCredits.value = false
        await fetchData()
        alert('积分更新成功')
    } catch (error) {
        console.error('Update credits failed:', error)
        alert(error.response?.data || '更新积分失败')
    } finally {
        actionLoading.value = false
    }
}

// 更新角色
const updateRole = async () => {
    if (!selectedUser.value || !newRole.value) {
        alert('请选择角色')
        return
    }

    actionLoading.value = true
    try {
        await axios.post('/api/admin/user/role', {
            user_id: selectedUser.value.id,
            role: newRole.value
        })
        
        showEditRole.value = false
        await fetchData()
        alert('角色更新成功')
    } catch (error) {
        console.error('Update role failed:', error)
        alert(error.response?.data || '更新角色失败')
    } finally {
        actionLoading.value = false
    }
}

// 删除用户
const deleteUser = async () => {
    if (!selectedUser.value) return

    actionLoading.value = true
    try {
        await axios.delete(`/api/admin/user/${selectedUser.value.id}`)
        
        showDeleteConfirm.value = false
        await fetchData()
        alert('用户删除成功')
    } catch (error) {
        console.error('Delete user failed:', error)
        alert(error.response?.data || '删除用户失败')
    } finally {
        actionLoading.value = false
    }
}

onMounted(() => {
    fetchData()
})
</script>
