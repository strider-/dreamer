var DS = DS || {};

DS.Web = {
    init: function() {
        var socket = io.connect("http://www-cdn-twitch.saltybet.com:8000");
        socket.on("message", this.getFightCard);
        this.getFightCard(null);
    },

    getFightCard: function(msg) {
        $.get("/api/f").done(function(data){
            $("body").text(JSON.stringify(data));
        });
    }
};

$(document).ready(function(){
    DS.Web.init();
});