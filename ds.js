var DS = DS || {};

DS.Web = {
    init: function() {
        var socket = io.connect("http://www-cdn-twitch.saltybet.com:8000");
        socket.on("message", this.getFightCard);
        // this.getFightCard(null);
    },

    getFightCard: function(msg) {
        $.get("/api/f").done(function(data){
            DS.Web.populateData($(".red"), data[0])
            DS.Web.populateData($(".blue"), data[1])
        });
    },

    populateData: function($elm, data) {
        data.Wins = data.Wins || []
        data.Losses = data.Losses || []

        $elm.find(".name").text(data.Fighter.Name)
        $elm.find(".label").text(data.Fighter.Elo)
        var $tblW = $elm.find("table.wins tbody").empty();
        var $tblL = $elm.find("table.losses tbody").empty();        

        $($tblW.closest('.fights').find('thead th')[0]).text(data.Wins.length + ' Wins')
        $($tblL.closest('.fights').find('thead th')[1]).text(data.Losses.length + ' Losses')

        $(data.Wins).sort(DS.Web.eloSort).each(function(index, item){
            DS.Web.appendRow($tblW, index, item)
        });
        $(data.Losses).sort(DS.Web.eloSort).each(function(index, item){
            DS.Web.appendRow($tblL, index, item)
        });        
    },

    appendRow: function($tbl, index, item) {
        var row = '<tr><td>' + item.Elo + '</td><td>' + item.Opponent + '</td></tr>';
        $tbl.append(row);        
    },

    eloSort: function(a, b) {
        return a.Elo == b.Elo ? 0 : (a.Elo > b.Elo) ? -1 : 1;
    }
};

$(document).ready(function(){
    DS.Web.init();
});