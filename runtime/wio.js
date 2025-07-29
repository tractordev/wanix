export function setupWio() {
  document.addEventListener('DOMContentLoaded', () => {
    let activeIframe = null;
    let isResizing = false;
    let isMoving = false;
    let startX, startY, startWidth, startHeight, startLeft, startTop;
    const border = 8;
    
    // Setup iframe controls for all existing and future iframes
    function setupIframeControls() {
      const iframes = document.querySelectorAll('iframe');
      iframes.forEach(iframe => {
        if (!iframe.hasAttribute('data-control-setup')) {
          iframe.style.pointerEvents = 'none';
          iframe.setAttribute('data-control-setup', 'true');
        }
      });
    }
    
    // Initial setup
    setupIframeControls();
    
    // Watch for new iframes being added
    const observer = new MutationObserver(mutations => {
      let needsSetup = false;
      
      mutations.forEach(mutation => {
        if (mutation.type === 'childList') {
          mutation.addedNodes.forEach(node => {
            if (node.tagName === 'IFRAME' || node.querySelectorAll) {
              needsSetup = true;
            }
          });
        }
      });
      
      if (needsSetup) {
        setupIframeControls();
      }
    });
    
    // Start observing with configuration
    observer.observe(document.body, {
      childList: true,
      subtree: true
    });
    
    document.addEventListener('mousedown', (e) => {
      // Check if click is on any iframe
      document.querySelectorAll('iframe').forEach(iframe => {
        const rect = iframe.getBoundingClientRect();
        
        // Check if click is within iframe boundaries
        if (e.clientX >= rect.left && e.clientX <= rect.right && 
            e.clientY >= rect.top && e.clientY <= rect.bottom) {
          
          // Prevent event from reaching background elements
          e.preventDefault();
          e.stopPropagation();
          
          // Calculate position relative to iframe
          const x = e.clientX - rect.left;
          const y = e.clientY - rect.top;
          
          // Check if click is in border or corner
          const isInLeftBorder = x < border;
          const isInRightBorder = x > rect.width - border;
          const isInTopBorder = y < border;
          const isInBottomBorder = y > rect.height - border;
          
          if ((isInLeftBorder || isInRightBorder) && (isInTopBorder || isInBottomBorder)) {
            // Corner click - resize
            isResizing = true;
            activeIframe = iframe;
            startX = e.clientX;
            startY = e.clientY;
            startWidth = rect.width;
            startHeight = rect.height;
          } else if (isInLeftBorder || isInRightBorder || isInTopBorder || isInBottomBorder) {
            // Border click - move
            isMoving = true;
            activeIframe = iframe;
            startX = e.clientX;
            startY = e.clientY;
            startLeft = rect.left;
            startTop = rect.top;
          }
        }
      });
    });
    
    document.addEventListener('mousemove', (e) => {
      if (isResizing && activeIframe || isMoving && activeIframe) {
        // Prevent event from reaching background elements
        e.preventDefault();
        e.stopPropagation();
        
        if (isResizing) {
          // Resize
          const width = startWidth + (e.clientX - startX);
          const height = startHeight + (e.clientY - startY);
          activeIframe.style.width = `${width}px`;
          activeIframe.style.height = `${height}px`;
        } else if (isMoving) {
          // Move
          const left = startLeft + (e.clientX - startX);
          const top = startTop + (e.clientY - startY);
          activeIframe.style.left = `${left}px`;
          activeIframe.style.top = `${top}px`;
        }
      }
    });
    
    document.addEventListener('mouseup', (e) => {
      if (isResizing || isMoving) {
        // Prevent event from reaching background elements
        e.preventDefault();
        e.stopPropagation();
      }
      isResizing = false;
      isMoving = false;
      activeIframe = null;
    });
  });
}
