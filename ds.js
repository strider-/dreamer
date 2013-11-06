var DS = DS || {};

DS.Web = {
    init: function() {
        var socket = io.connect("http://www-cdn-twitch.saltybet.com:8000");
        socket.on("message", function(data){
            window.reload();
        });
    }
};

$(document).ready(function(){
    DS.Web.init();
});