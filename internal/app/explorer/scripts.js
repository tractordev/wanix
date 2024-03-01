document.addEventListener('DOMContentLoaded', function() {
    const deleteButton = document.getElementById('button-delete-file');
    deleteButton.addEventListener('click', function() {
        // ahow confirmation dialog
        const userResponse = confirm('Are you sure you want to delete this file?');
        
        if (userResponse) {
            alert('You clicked OK!');
        } else {
            alert('You clicked Cancel!');
        }
    });
});