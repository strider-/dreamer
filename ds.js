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
        var $tblL = $elm.find("table.losses tbody")
        $tblW.empty();
        $tblL.empty();

        $(data.Wins).sort(DS.Web.eloSort).each(function(index, item){
            var row = '<tr><td>' + item.Elo + '</td><td>' + item.Opponent + '</td></tr>';
            $tblW.append(row);
        });
        $(data.Losses).sort(DS.Web.eloSort).each(function(index, item){
            var row = '<tr><td>' + item.Elo + '</td><td>' + item.Opponent + '</td></tr>';
            $tblL.append(row);
        });        
    },

    eloSort: function(a, b) {
        return a.Elo == b.Elo ? 0 : (a.Elo > b.Elo) ? -1 : 1;
    }
};

$(document).ready(function(){
    DS.Web.init();
});