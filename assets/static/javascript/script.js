document.addEventListener('DOMContentLoaded', function () {
    var inputs = document.getElementsByTagName('input');
    for (var i = 0; i < inputs.length; i++) {
        inputs[i].addEventListener('focus', function() {
            this.style.borderColor = '#0056b3';
        });
        inputs[i].addEventListener('blur', function() {
            this.style.borderColor = '#ccc';
        });
    }
});