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

    // Form submission feedback
    const forms = document.querySelectorAll('form');
    forms.forEach(form => {
        form.addEventListener('submit', function(e) {
            const button = form.querySelector('button[type="submit"]');
            if (button) {
                button.textContent = 'Processing...';
                button.disabled = true;
                button.style.opacity = '0.7';
                button.style.cursor = 'wait';

                // Add a helpful message for long-running operations
                const formAction = form.getAttribute('action');
                if (formAction && (formAction.includes('cyclonedx-bom') || formAction.includes('sbom-process'))) {
                    let helpText = form.querySelector('.processing-help');
                    if (!helpText) {
                        helpText = document.createElement('div');
                        helpText.className = 'processing-help';
                        helpText.style.cssText = 'margin-top: 1rem; padding: 1rem; background: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 0.5rem; font-size: 0.875rem; color: #92400e;';
                        helpText.innerHTML = '<strong>⏳ This may take several minutes</strong><br>Fetching CVE data and building dependency graphs...';
                        button.parentElement.appendChild(helpText);
                    }
                }
            }
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