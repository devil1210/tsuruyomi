/* tsuruyomi-v5-wallclock-timepicker */
/* tsuruyomi-v4-symmetrical-splash */
/* tsuruyomi-v3-fullbleed */
if(!self.define){let e,n={};const t=(t,r)=>(t=new URL(t+".js",r).href,n[t]||new Promise(n=>{if("document"in self){const e=document.createElement("script");e.src=t,e.onload=n,document.head.appendChild(e)}else e=t,importScripts(t),n()}).then(()=>{let e=n[t];if(!e)throw new Error(`Module ${t} didn’t register its module`);return e}));self.define=(r,a)=>{const s=e||("document"in self?document.currentScript.src:"")||location.href;if(n[s])return;let o={};const i=e=>t(e,s),c={module:{uri:s},exports:o,require:i};n[s]=Promise.all(r.map(e=>c[e]||i(e))).then(e=>(a(...e),o))}}define(["./workbox-c494150e"],function(e){"use strict";self.skipWaiting(),e.clientsClaim(),e.registerRoute(({url:e})=>{const{pathname:n}=e;return n.match(/\/chapter\/[0-9]+\/page\/[0-9]+/g)},new e.CacheFirst({cacheName:"image-cache-chapter-pages",matchOptions:{ignoreVary:!0},plugins:[new e.ExpirationPlugin({purgeOnQuotaError:!0,maxAgeSeconds:86400,maxEntries:2500,matchOptions:{ignoreVary:!0}}),new e.CacheableResponsePlugin({statuses:[0,200]})]}),"GET"),e.registerRoute(({url:e})=>{const{pathname:n}=e;return n.match(/\/manga\/[0-9]+\/thumbnail/g)},new e.CacheFirst({cacheName:"image-cache-manga-thumbnails",matchOptions:{ignoreVary:!0},plugins:[new e.ExpirationPlugin({purgeOnQuotaError:!0,maxAgeSeconds:86400,maxEntries:5e3,matchOptions:{ignoreVary:!0}}),new e.CacheableResponsePlugin({statuses:[0,200]})]}),"GET"),e.registerRoute(({url:e})=>{const{pathname:n}=e;return n.includes("/extension/icon/")},new e.CacheFirst({cacheName:"image-cache-extension-icons",matchOptions:{ignoreVary:!0},plugins:[new e.ExpirationPlugin({purgeOnQuotaError:!0,maxAgeSeconds:31536e3,maxEntries:300,matchOptions:{ignoreVary:!0}}),new e.CacheableResponsePlugin({statuses:[0,200]})]}),"GET"),e.registerRoute(({request:e})=>"image"===e.destination,new e.CacheFirst({cacheName:"image-cache-other-tsuruyomi",matchOptions:{ignoreVary:!0},plugins:[new e.ExpirationPlugin({purgeOnQuotaError:!0,maxAgeSeconds:345600,maxEntries:500,matchOptions:{ignoreVary:!0}}),new e.CacheableResponsePlugin({statuses:[0,200]})]}),"GET")});

/* tsuruyomi-cache-bust-v4 */
self.addEventListener("activate", function(e) {
  self.skipWaiting();
  e.waitUntil(
    caches.keys().then(function(keys) {
      return Promise.all(
        keys.map(function(k) {
          console.log("[Tsuruyomi SW] Purging stale cache:", k);
          return caches.delete(k);
        })
      );
    }).then(function() {
      return self.clients.claim();
    })
  );
});
