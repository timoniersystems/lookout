document.addEventListener('DOMContentLoaded', function () {
    // File input handling - show selected filename
    const fileInputs = document.querySelectorAll('input[type="file"]');
    fileInputs.forEach(input => {
        input.addEventListener('change', function(e) {
            const fileName = e.target.files[0]?.name;
            if (fileName) {
                // Create or update filename display
                let display = input.parentElement.querySelector('.file-name-display');
                if (!display) {
                    display = document.createElement('div');
                    display.className = 'file-name-display';
                    input.parentElement.appendChild(display);
                }
                display.textContent = `📄 ${fileName}`;
                display.style.cssText = 'margin-top: 0.5rem; font-size: 0.875rem; color: #10b981; font-weight: 500;';
            }
        });
    });

    // Form submission with error handling via fetch
    const forms = document.querySelectorAll('form');
    forms.forEach(form => {
        form.addEventListener('submit', function(e) {
            e.preventDefault();

            const button = form.querySelector('button[type="submit"]');
            const originalText = button ? button.textContent : '';

            if (button) {
                button.textContent = 'Processing...';
                button.disabled = true;
                button.style.opacity = '0.7';
                button.style.cursor = 'wait';
            }

            // Add helpful message for long-running SBOM operations
            const formAction = form.getAttribute('action');
            let helpText = null;
            if (formAction && (formAction.includes('cyclonedx-bom') || formAction.includes('sbom-process'))) {
                helpText = form.querySelector('.processing-help');
                if (!helpText) {
                    helpText = document.createElement('div');
                    helpText.className = 'processing-help';
                    helpText.style.cssText = 'margin-top: 1rem; padding: 1rem; background: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 0.5rem; font-size: 0.875rem; color: #92400e;';
                    helpText.innerHTML = '<strong>⏳ This may take several minutes</strong><br>Fetching CVE data and building dependency graphs...';
                    button.parentElement.appendChild(helpText);
                }
            }

            const formData = new FormData(form);

            fetch(form.action, {
                method: 'POST',
                body: formData,
            })
            .then(response => {
                if (!response.ok) {
                    return response.json().then(data => {
                        throw new Error(data.error || 'An unexpected error occurred');
                    });
                }
                const contentType = response.headers.get('content-type') || '';
                if (contentType.includes('application/json')) {
                    return response.json().then(data => {
                        if (data.redirect) {
                            // Navigate as a real browser request so cached basic auth
                            // credentials are preserved for the SSE connection.
                            window.location.href = data.redirect;
                        }
                    });
                }
                // Success - replace the page with the response HTML
                return response.text().then(html => {
                    document.open();
                    document.write(html);
                    document.close();
                });
            })
            .catch(error => {
                // Show error in modal
                showErrorModal(error.message);

                // Reset button state
                if (button) {
                    button.textContent = originalText;
                    button.disabled = false;
                    button.style.opacity = '';
                    button.style.cursor = '';
                }

                // Remove help text if present
                if (helpText) {
                    helpText.remove();
                }
            });
        });
    });

    // CVE ID input validation feedback
    const cveInput = document.getElementById('cveID');
    if (cveInput) {
        cveInput.addEventListener('input', function(e) {
            const value = e.target.value;
            const pattern = /^CVE-\d{4}-\d{4,}$/;

            if (value && !pattern.test(value)) {
                e.target.style.borderColor = '#ef4444';
            } else {
                e.target.style.borderColor = '';
            }
        });
    }

    // Add smooth scroll behavior
    document.documentElement.style.scrollBehavior = 'smooth';
});

function showErrorModal(message) {
    const modal = document.getElementById('errorModal');
    const messageEl = document.getElementById('errorModalMessage');
    if (modal && messageEl) {
        messageEl.textContent = message;
        modal.style.display = 'flex';
    }
}

function closeErrorModal() {
    const modal = document.getElementById('errorModal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// Close modal on overlay click
document.addEventListener('click', function(e) {
    if (e.target.classList.contains('error-modal-overlay')) {
        closeErrorModal();
    }
});

// Close modal on Escape key
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
        closeErrorModal();
    }
});