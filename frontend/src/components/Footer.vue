<template>
  <footer class="footer">
    <router-link to="/">Home</router-link>
    <router-link to="/help">Help</router-link>
    <a href="https://github.com/zzhirong/contextdict" target="_blank">GitHub</a>
  </footer>
</template>

<style scoped>
.footer {
  margin-top: 2rem;
  padding: 0.5rem;
  display: flex;
  justify-content: center;
  gap: 2rem;
  width: 100%;
  border-top: 1px solid #eee;
}

.footer a {
  color: #4CAF50;
  text-decoration: none;
}

.footer a:hover {
  text-decoration: underline;
}
</style>
