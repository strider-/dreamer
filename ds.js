var DS = DS || {};

DS.Web = {
    init: function() {
        var socket = io.connect("http://www-cdn-twitch.saltybet.com:8000");
        socket.on("message", this.getFightCard);
    },

    getFightCard: function(data) {        
        $.get("http://www.saltybet.com/state.json").done(function(data){
            $("body").text(data);
        });
    }
};

$(document).ready(function(){
    DS.Web.init();
});