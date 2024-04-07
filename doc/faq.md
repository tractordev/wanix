# Troubleshooting

## Unregister service worker
If you notice buggy behavior with WANIX, unregistering the service worker can be a good first troubleshooting step. 

### Chromium Browsers (Chrome, Arc...)
- Open the Developer Console
- Select the **Application** tab
- In the left sidebar, select **Service workers**
- For the wanix-bootloader service worker, select **Unregister**

## Clear all data
You can also try clearing the site data, which unregisters the service worker and clears the cache.

### Chromium Browsers (Chrome, Arc...)
- Open the Developer Console
- Select the **Application** tab
- In the left sidebar, select **Storage**
- Scroll down and click the **Clear site data** button
