// Open external links in new tab
document.addEventListener('DOMContentLoaded', function() {
  // Get all links
  var links = document.querySelectorAll('a[href^="http"]');
  
  links.forEach(function(link) {
    // Check if link is external (not same origin)
    if (link.hostname !== window.location.hostname) {
      link.setAttribute('target', '_blank');
      link.setAttribute('rel', 'noopener noreferrer');
    }
  });
});
