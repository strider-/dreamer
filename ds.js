var DS = DS || {};

DS.Web = {
    init: function() {
        var socket = io.connect("http://www-cdn-twitch.saltybet.com:8000");
        socket.on("message", this.getFightCard);
        this.getFightCard(null);
    },

    getFightCard: function(msg) {
        $.get("/api/f").done(function(data){
            var $red = $(".red")
            var $blue = $(".blue")

            $red.find("h1").text(data[0].Fighter.Name)
            $blue.find("h1").text(data[1].Fighter.Name)
        });
    }
};

$(document).ready(function(){
    DS.Web.init();
});