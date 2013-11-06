var DS = DS || {};

DS.Web = {
    init: function() {
        var socket = io.connect("http://www-cdn-twitch.saltybet.com:8000");
        socket.on("message", this.getFightCard);
        this.getFightCard(null);
    },

    getFightCard: function(msg) {
        $.get("/api/f").done(function(data){
            DS.Web.populateData($(".red"), data[0])
            DS.Web.populateData($(".blue"), data[1])
        });
    },

    populateData: function($elm, data) {
        $elm.find("h1").text(data.Fighter.Name)
        var $tblW = $elm.find("table.wins tbody");
        $tblW.empty();
        
        $(data.Wins).each(function(index, item){
            var row = '<tr><td>' + item.Opponent + '</td></tr>';
            $tblW.append(row);
        });
    }
};

$(document).ready(function(){
    DS.Web.init();
});