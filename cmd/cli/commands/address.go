package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.33.cn/chain33/chain33/account"
	jsonrpc "gitlab.33.cn/chain33/chain33/rpc"
	"gitlab.33.cn/chain33/chain33/types"
)

func AddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addr",
		Short: "Address managerment",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		AddressViewCmd(),
		GetAddressCmd(),
		ColdAddressOfMinerCmd(),
	)

	return cmd
}

// view
func AddressViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View transactions of address",
		Run:   viewAddress,
	}
	addAddrViewFlags(cmd)
	return cmd
}

func addAddrViewFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("addr", "a", "", "account address")
	cmd.MarkFlagRequired("addr")
}

func viewAddress(cmd *cobra.Command, args []string) {
	rpcLaddr, _ := cmd.Flags().GetString("rpc_laddr")
	addr, _ := cmd.Flags().GetString("addr")
	params := types.ReqAddr{
		Addr: addr,
	}

	var res types.AddrOverview
	ctx := NewRPCCtx(rpcLaddr, "Chain33.GetAddrOverview", params, &res)
	ctx.SetResultCb(parseAddrOverview)
	ctx.Run()
}

func parseAddrOverview(view interface{}) (interface{}, error) {
	res := view.(*types.AddrOverview)
	Balance := strconv.FormatFloat(float64(res.GetBalance())/float64(types.Coin), 'f', 4, 64)
	Reciver := strconv.FormatFloat(float64(res.GetReciver())/float64(types.Coin), 'f', 4, 64)
	addrOverview := &AddrOverviewResult{
		Balance: Balance,
		Reciver: Reciver,
		TxCount: res.GetTxCount(),
	}
	return addrOverview, nil
}

// get
func GetAddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get address of executer",
		Run:   getAddrByExec,
	}
	addGetAddrFlags(cmd)
	return cmd
}

func addGetAddrFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("exec", "e", "", `executer name ("none", "coins", "hashlock", "retrieve", "ticket", "token" and "trade" supported)`)
	cmd.MarkFlagRequired("exec")
}

func getAddrByExec(cmd *cobra.Command, args []string) {
	execer, _ := cmd.Flags().GetString("exec")
	switch execer {
	case "none", "coins", "hashlock", "retrieve", "ticket", "token", "trade":
		addrResult := account.ExecAddress(execer)
		result := addrResult.String()
		data, err := json.MarshalIndent(result, "", "    ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}

		fmt.Println(string(data))

	default:
		fmt.Println("only none, coins, hashlock, retrieve, ticket, token, trade supported")
	}
}

// cold
func ColdAddressOfMinerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cold",
		Short: "Get cold wallet address of miner",
		Run:   coldAddressOfMiner,
	}
	addColdAddressOfMinerFlags(cmd)
	return cmd
}

func addColdAddressOfMinerFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("miner", "m", "", "miner address")
	cmd.MarkFlagRequired("miner")
}

func coldAddressOfMiner(cmd *cobra.Command, args []string) {
	rpcLaddr, _ := cmd.Flags().GetString("rpc_laddr")
	addr, _ := cmd.Flags().GetString("miner")
	reqaddr := &types.ReqString{
		Data: addr,
	}
	var params jsonrpc.Query4Cli
	params.Execer = "ticket"
	params.FuncName = "MinerSourceList"
	params.Payload = reqaddr

	var res types.Message
	ctx := NewRPCCtx(rpcLaddr, "Chain33.Query", params, &res)
	ctx.Run()
}