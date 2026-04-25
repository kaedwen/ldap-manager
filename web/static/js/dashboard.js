// Dashboard JavaScript

// Wait for DOM to be ready
document.addEventListener('DOMContentLoaded', function() {
    const copyBtn = document.getElementById('copy-btn');
    const input = document.getElementById('reset-link-input');

    // Add click listener to input for selection
    if (input) {
        input.addEventListener('click', function() {
            this.select();
        });
    }

    // Add click listener to copy button
    if (copyBtn) {
        copyBtn.addEventListener('click', async function(e) {
            e.preventDefault();

            const status = document.getElementById('copy-status');

            if (!input) {
                console.error('Input element not found');
                return;
            }

            const textToCopy = input.value;

            // Try modern Clipboard API first
            if (navigator.clipboard && navigator.clipboard.writeText) {
                try {
                    await navigator.clipboard.writeText(textToCopy);
                    console.log('Clipboard API success');

                    if (status) status.textContent = '✓ Copied to clipboard!';
                    copyBtn.textContent = 'Copied!';
                    copyBtn.style.background = '#059669';
                    setTimeout(() => {
                        copyBtn.textContent = 'Copy';
                        copyBtn.style.background = '';
                    }, 2000);
                    return;
                } catch (err) {
                    console.error('Clipboard API failed:', err);
                    // Fall through to execCommand fallback
                }
            }

            // Fallback to execCommand
            input.focus();
            input.select();

            try {
                const success = document.execCommand('copy');
                console.log('execCommand success:', success);

                if (success) {
                    if (status) status.textContent = '✓ Copied to clipboard!';
                    copyBtn.textContent = 'Copied!';
                    copyBtn.style.background = '#059669';
                    setTimeout(() => {
                        copyBtn.textContent = 'Copy';
                        copyBtn.style.background = '';
                    }, 2000);
                } else {
                    if (status) {
                        status.textContent = 'Please copy manually (Ctrl+C or Cmd+C)';
                        status.style.color = '#dc2626';
                    }
                }
            } catch (err) {
                console.error('execCommand failed:', err);
                if (status) {
                    status.textContent = 'Please copy manually (Ctrl+C or Cmd+C)';
                    status.style.color = '#dc2626';
                }
            }
        });
    }
});
