import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('../views/Login.vue')
    },
    {
      path: '/',
      component: () => import('../views/Layout.vue'),
      children: [
        {
          path: '',
          name: 'Buckets',
          component: () => import('../views/Buckets.vue')
        },
        {
          path: 'bucket/:name',
          name: 'Objects',
          component: () => import('../views/Objects.vue')
        },
        {
          path: 'tools',
          name: 'Tools',
          component: () => import('../views/Tools.vue')
        }
      ]
    }
  ]
})

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (to.name !== 'Login' && !auth.isLoggedIn) {
    next({ name: 'Login' })
  } else {
    next()
  }
})

export default router
