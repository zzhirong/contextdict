import { createRouter, createWebHistory } from 'vue-router'
import Home from '../views/Home.vue'
import Help from '../views/Help.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'Home',
      component: Home
    },
    {
      path: '/help',
      name: 'Help',
      component: Help
    }
  ]
})

export default router