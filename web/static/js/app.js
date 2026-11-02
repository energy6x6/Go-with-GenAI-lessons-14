// Render server-provided error fragments while preserving their HTTP status.
document.addEventListener('htmx:beforeSwap', function (event) {
  if (event.detail.xhr.status >= 400 && event.detail.target.id === 'feedback') {
    event.detail.shouldSwap = true;
    event.detail.isError = false;
  }
});
document.addEventListener('htmx:afterSwap', function (event) {
  if (event.detail.target.id === 'book-list') {
    document.getElementById('pagination').hidden = true;
    document.getElementById('search-help').hidden = false;
  }
});
