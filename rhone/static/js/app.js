// Rhone - Minimal JavaScript
// Most interactivity is handled by HTMX

// Configure HTMX
document.addEventListener('DOMContentLoaded', function() {
    // Log HTMX errors in development
    document.body.addEventListener('htmx:responseError', function(event) {
        console.error('HTMX error:', event.detail);
    });

    // Add loading indicator on HTMX requests
    document.body.addEventListener('htmx:beforeRequest', function(event) {
        document.body.classList.add('htmx-request');
    });

    document.body.addEventListener('htmx:afterRequest', function(event) {
        document.body.classList.remove('htmx-request');
    });
});
